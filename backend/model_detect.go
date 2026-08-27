package backend

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	probePort    = 3702 // WSD discovery port
	probeWait    = 4 * time.Second
	getTimeout   = 8 * time.Second
	maxMetaSize  = 1 << 20
	discoverWait = 4 * time.Second
)

// ModelInfo describes the detected scanner model.
type ModelInfo struct {
	Model     string `json:"model"`     // m7300fdn | m7300fdw | "" (unknown)
	ModelName string `json:"modelName"` // e.g. "M7300FDN series"
	Host      string `json:"host"`
	Source    string `json:"source"` // wsd | config | manual
	Error     string `json:"error"`
}

var (
	xaddrsRe    = regexp.MustCompile(`(?s)<\w+:XAddrs[^>]*>\s*([^<]+?)\s*</\w+:XAddrs>`)
	modelNameRe = regexp.MustCompile(`(?s)<\w+:ModelName[^>]*>\s*([^<]+?)\s*</\w+:ModelName>`)
	friendlyRe  = regexp.MustCompile(`(?s)<\w+:FriendlyName[^>]*>\s*([^<]+?)\s*</\w+:FriendlyName>`)
)

func randomUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// soapEnvelope mirrors the driver's soap_body() in
// scanner/src/protocol/wsd_client.cpp (the device ignores malformed probes).
func soapEnvelope(to, action, body string) []byte {
	env := "<?xml version=\"1.0\" encoding=\"utf-8\"?>\r\n" +
		"<soap:Envelope \r\n" +
		"xmlns:soap=\"http://www.w3.org/2003/05/soap-envelope\"\r\n" +
		"xmlns:wsa=\"http://schemas.xmlsoap.org/ws/2004/08/addressing\"\r\n" +
		"xmlns:UNS1=\"http://www.microsoft.com/windows/test/testdevice/11/2005\"\r\n" +
		"xmlns:sca=\"http://schemas.microsoft.com/windows/2006/08/wdp/scan\">\r\n" +
		"<soap:Header>\r\n" +
		"<wsa:To>" + to + "</wsa:To>\r\n" +
		"<wsa:Action>" + action + "</wsa:Action>\r\n" +
		"<wsa:MessageID>urn:uuid:" + randomUUID() + "</wsa:MessageID>\r\n" +
		"<wsa:ReplyTo>\r\n" +
		"<wsa:Address>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</wsa:Address>\r\n" +
		"</wsa:ReplyTo>\r\n" +
		"<wsa:From>\r\n<wsa:Address>urn:uuid:" + randomUUID() + "</wsa:Address>\r\n</wsa:From>\r\n" +
		"<UNS1:ServiceIdentifier>uri:scn</UNS1:ServiceIdentifier>\r\n" +
		"</soap:Header>\r\n" +
		"<soap:Body>\r\n" + body + "\r\n</soap:Body>\r\n" +
		"</soap:Envelope>\r\n"
	return []byte(env)
}

// wsdProbe asks the device for its scan endpoint (XAddrs) via UDP 3702.
func wsdProbe(host string) string {
	conn, err := net.DialTimeout("udp", net.JoinHostPort(host, "3702"), probeWait)
	if err != nil {
		return ""
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(probeWait))
	probe := soapEnvelope(
		"urn:schemas-xmlsoap-org:ws:2005:04:discovery",
		"http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe",
		`<wsd:Probe xmlns:wsd="http://schemas.xmlsoap.org/ws/2005/04/discovery">`+
			`<wsd:Types>wsd:ScanDeviceType</wsd:Types></wsd:Probe>`)
	for attempt := 0; attempt < 3; attempt++ {
		if _, err := conn.Write(probe); err != nil {
			break
		}
		buf := make([]byte, 65536)
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		if m := xaddrsRe.FindSubmatch(buf[:n]); m != nil {
			return strings.TrimSpace(string(m[1]))
		}
	}
	return ""
}

// wsTransferGet fetches the device metadata and returns the ModelName and
// FriendlyName (e.g. "M7300FDN series" / "Pantum-A6E2CB (M7300FDN series)").
// The XAddrs host is a device hostname that is usually not DNS-resolvable, so
// the probed IP is substituted (mirrors parse_endpoint_url in the driver).
func wsTransferGet(xaddrs, fallbackHost string) (string, string) {
	u, err := url.Parse(xaddrs)
	if err != nil {
		return "", ""
	}
	port := u.Port()
	if port == "" {
		port = "5357"
	}
	u.Host = net.JoinHostPort(fallbackHost, port)
	client := &http.Client{Timeout: getTimeout}
	env := soapEnvelope(
		u.String(),
		"http://schemas.xmlsoap.org/ws/2004/09/transfer/Get",
		`<Get xmlns="http://schemas.xmlsoap.org/ws/2004/09/transfer"/>`)
	resp, err := client.Post(u.String(), "application/soap+xml; charset=utf-8", bytes.NewReader(env))
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxMetaSize))
	model, friendly := "", ""
	if m := modelNameRe.FindSubmatch(body); m != nil {
		model = strings.TrimSpace(string(m[1]))
	}
	if f := friendlyRe.FindSubmatch(body); f != nil {
		friendly = strings.TrimSpace(string(f[1]))
	}
	return model, friendly
}

// modelFromName maps a device-reported name to a driver model id.
func modelFromName(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "m7300fdw"):
		return "m7300fdw"
	case strings.Contains(n, "m7300fdn"):
		return "m7300fdn"
	}
	return ""
}

// subnetBroadcast computes the IPv4 broadcast address of an interface.
func subnetBroadcast(iface *net.Interface) (*net.UDPAddr, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.To4() == nil {
			continue
		}
		ip4 := ipn.IP.To4()
		mask := ipn.Mask
		bcast := net.IPv4(ip4[0]|^mask[0], ip4[1]|^mask[1], ip4[2]|^mask[2], ip4[3]|^mask[3])
		return &net.UDPAddr{IP: bcast, Port: probePort}, nil
	}
	return nil, fmt.Errorf("no IPv4 address on %s", iface.Name)
}

// subnetHosts enumerates the usable host addresses of an interface's subnet.
// The network size is derived from the interface's own netmask (never
// hardcoded). At most `limit` addresses are returned to bound scan time on
// very large networks.
func subnetHosts(ipn *net.IPNet, limit int) []net.IP {
	ip4 := ipn.IP.To4()
	if ip4 == nil {
		return nil
	}
	ones, bits := ipn.Mask.Size()
	if ones == 0 {
		return nil
	}
	total := uint32(1) << uint(bits-ones) // includes network + broadcast
	if total < 3 {
		return nil
	}
	usable := total - 2
	if usable > uint32(limit) {
		usable = uint32(limit)
	}
	base := binary.BigEndian.Uint32(ip4.Mask(ipn.Mask))
	hosts := make([]net.IP, 0, usable)
	for i := uint32(1); i <= usable; i++ {
		h := base + i
		hosts = append(hosts, net.IPv4(byte(h>>24), byte(h>>16), byte(h>>8), byte(h)))
	}
	return hosts
}

const maxSubnetSweep = 1024 // cap for hosts probed per interface

func isUsbHost(host string) bool {
	return host == "usb" || strings.HasPrefix(host, "usb:")
}

func readSysfsTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// DiscoverUsbDevices lists Pantum USB scanners (VID 0x232B) from sysfs.
func DiscoverUsbDevices() []ModelInfo {
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		return nil
	}
	out := []ModelInfo{}
	seen := map[string]bool{}
	for _, entry := range entries {
		dir := filepath.Join("/sys/bus/usb/devices", entry.Name())
		if strings.ToLower(readSysfsTrim(filepath.Join(dir, "idVendor"))) != "232b" {
			continue
		}
		bus := readSysfsTrim(filepath.Join(dir, "busnum"))
		addr := readSysfsTrim(filepath.Join(dir, "devnum"))
		if bus == "" || addr == "" {
			continue
		}
		host := "usb:" + bus + ":" + addr
		if seen[host] {
			continue
		}
		seen[host] = true
		product := readSysfsTrim(filepath.Join(dir, "product"))
		model := modelFromName(product)
		if model == "" {
			model = "m7300fdn"
		}
		name := product
		if name == "" {
			name = "Pantum USB scanner"
		}
		out = append(out, ModelInfo{
			Model:     model,
			ModelName: name,
			Host:      host,
			Source:    "usb",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}

// DiscoverDevices scans the local network with WSD Probes: multicast and
// broadcast first, then a unicast sweep of every host in each local IPv4
// subnet (netmask taken dynamically from the interface itself). Every device
// that answers is queried for its model; only supported M7300FDN / M7300FDW
// models are reported. When nothing answers, hosts configured in the SANE
// config files are probed unicast as a fallback.
func DiscoverDevices() []ModelInfo {
	probe := soapEnvelope(
		"urn:schemas-xmlsoap-org:ws:2005:04:discovery",
		"http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe",
		`<wsd:Probe xmlns:wsd="http://schemas.xmlsoap.org/ws/2005/04/discovery">`+
			`<wsd:Types>wsd:ScanDeviceType</wsd:Types></wsd:Probe>`)

	// Create the socket manually so SO_BROADCAST + IP_ADD_MEMBERSHIP can be
	// set, then wrap it so read deadlines work (net.Conn.File() would switch
	// the socket back to blocking mode and break deadlines).
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return nil
	}
	_ = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	_ = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	_ = syscall.SetsockoptIPMreq(fd, syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP,
		&syscall.IPMreq{Multiaddr: [4]byte{239, 255, 255, 250}})
	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Port: probePort}); err != nil {
		_ = syscall.Close(fd)
		return nil
	}
	file := os.NewFile(uintptr(fd), "wsd-discovery")
	conn, err := net.FilePacketConn(file)
	file.Close()
	if err != nil {
		_ = syscall.Close(fd)
		return nil
	}
	defer conn.Close()
	udp := conn.(*net.UDPConn)

	group := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: probePort}
	broadcastAll := &net.UDPAddr{IP: net.IPv4bcast, Port: probePort}
	_ = udp.SetWriteDeadline(time.Now().Add(4 * time.Second))

	// Multicast + broadcast probes.
	_, _ = udp.WriteToUDP(probe, group)
	_, _ = udp.WriteToUDP(probe, broadcastAll)

	// Unicast sweep of every host in each local subnet (dynamic netmask).
	if ifaces, _ := net.Interfaces(); len(ifaces) > 0 {
		for i := range ifaces {
			ifc := &ifaces[i]
			if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
				continue
			}
			if bcast, berr := subnetBroadcast(ifc); berr == nil {
				_, _ = udp.WriteToUDP(probe, bcast)
			}
			addrs, _ := ifc.Addrs()
			for _, a := range addrs {
				ipn, ok := a.(*net.IPNet)
				if !ok || ipn.IP.To4() == nil {
					continue
				}
				own := ipn.IP.To4().String()
				for _, h := range subnetHosts(ipn, maxSubnetSweep) {
					if h.String() == own {
						continue
					}
					_, _ = udp.WriteToUDP(probe, &net.UDPAddr{IP: h, Port: probePort})
				}
			}
		}
	}

	found := map[string]string{} // sender IP -> XAddrs
	_ = udp.SetReadDeadline(time.Now().Add(discoverWait))
	buf := make([]byte, 65536)
	for {
		n, addr, err := udp.ReadFromUDP(buf)
		if err != nil {
			break
		}
		if m := xaddrsRe.FindSubmatch(buf[:n]); m != nil {
			ip := addr.IP.String()
			if _, ok := found[ip]; !ok {
				found[ip] = string(m[1])
			}
		}
	}

	out := collectSupportedDevices(found)
	out = append(out, DiscoverUsbDevices()...)

	// Fallback: probe hosts from the SANE config files unicast.
	if len(out) == 0 {
		fallback := map[string]string{}
		for _, model := range []string{"m7300fdn", "m7300fdw"} {
			for _, h := range readConfHosts(model) {
				if xaddr := wsdProbeQuick(h); xaddr != "" {
					fallback[h] = xaddr
				}
			}
		}
		out = collectSupportedDevices(fallback)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}

// collectSupportedDevices queries metadata for each discovered endpoint and
// keeps only devices that report a supported model. Queries run concurrently.
func collectSupportedDevices(endpoints map[string]string) []ModelInfo {
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := []ModelInfo{}
	for ip, xaddr := range endpoints {
		wg.Add(1)
		go func(ip, xaddr string) {
			defer wg.Done()
			modelName, _ := wsTransferGet(xaddr, ip)
			if modelName == "" {
				return
			}
			model := modelFromName(modelName)
			if model == "" {
				return
			}
			mu.Lock()
			out = append(out, ModelInfo{Model: model, ModelName: modelName, Host: ip, Source: "wsd"})
			mu.Unlock()
		}(ip, xaddr)
	}
	wg.Wait()
	return out
}

// wsdProbeQuick is a short single-attempt variant of wsdProbe for discovery.
func wsdProbeQuick(host string) string {
	conn, err := net.DialTimeout("udp", net.JoinHostPort(host, "3702"), 2*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	probe := soapEnvelope(
		"urn:schemas-xmlsoap-org:ws:2005:04:discovery",
		"http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe",
		`<wsd:Probe xmlns:wsd="http://schemas.xmlsoap.org/ws/2005/04/discovery">`+
			`<wsd:Types>wsd:ScanDeviceType</wsd:Types></wsd:Probe>`)
	if _, err := conn.Write(probe); err != nil {
		return ""
	}
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		return ""
	}
	if m := xaddrsRe.FindSubmatch(buf[:n]); m != nil {
		return strings.TrimSpace(string(m[1]))
	}
	return ""
}

// readConfHosts parses /etc/sane.d/<model>.conf (SANE_CONFIG_DIR aware).
func readConfHosts(model string) []string {
	dir := os.Getenv("SANE_CONFIG_DIR")
	if dir == "" {
		dir = "/etc/sane.d"
	} else if i := strings.IndexByte(dir, ':'); i >= 0 {
		dir = dir[:i]
	}
	data, err := os.ReadFile(filepath.Join(dir, model+".conf"))
	if err != nil {
		return nil
	}
	var hosts []string
	for _, raw := range strings.Split(string(data), "\n") {
		if i := strings.IndexByte(raw, '#'); i >= 0 {
			raw = raw[:i]
		}
		h := strings.TrimSpace(raw)
		if h != "" {
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// DetectModel identifies the scanner model. Detection order:
//  1. WSD live probe of the given host (or the hosts configured in the SANE
//     config files) and parse the device metadata ModelName.
//  2. Fall back to whichever of m7300fdn.conf / m7300fdw.conf has hosts.
//
// Returns a ModelInfo with Source reflecting how it was identified.
func DetectModel(host string) ModelInfo {
	if isUsbHost(host) {
		for _, usb := range DiscoverUsbDevices() {
			if host == "usb" || usb.Host == host {
				usb.Host = host
				return usb
			}
		}
		return ModelInfo{
			Model:     "m7300fdn",
			ModelName: "Pantum USB scanner",
			Host:      host,
			Source:    "usb",
		}
	}

	hosts := []string{}
	if host != "" {
		hosts = append(hosts, host)
	}
	for _, model := range []string{"m7300fdn", "m7300fdw"} {
		hosts = append(hosts, readConfHosts(model)...)
	}

	seen := map[string]bool{}
	for _, h := range hosts {
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		if xaddrs := wsdProbe(h); xaddrs != "" {
			if modelName, friendly := wsTransferGet(xaddrs, h); modelName != "" {
				if model := modelFromName(modelName); model != "" {
					return ModelInfo{
						Model:     model,
						ModelName: modelName,
						Host:      h,
						Source:    "wsd",
					}
				}
				// metadata reachable but name unknown; keep friendly name for UI
				if friendly != "" {
					modelName = friendly
				}
			}
		}
	}

	// Fallback: which config file has a device configured?
	for _, model := range []string{"m7300fdn", "m7300fdw"} {
		if hosts := readConfHosts(model); len(hosts) > 0 {
			info := ModelInfo{Model: model, ModelName: model + " series", Source: "config"}
			if host != "" {
				info.Host = host
			} else {
				info.Host = hosts[0]
			}
			return info
		}
	}
	return ModelInfo{Source: "config", Error: "no device found"}
}

// FindBinary locates the <model>-scan executable. Candidates are checked in
// order; the one with the newest modification time wins so a freshly built
// driver binary is preferred over a stale /usr/local/bin copy.
func FindBinary(model string) string {
	name := model + "-scan"
	home, _ := os.UserHomeDir()
	candidates := []string{
		"", // placeholder for PATH lookup
		"/usr/local/bin/" + name,
		"/usr/bin/" + name,
		"/opt/" + name,
		filepath.Join(home, "bin", name),
		filepath.Join(home, "下载", "pantum", "m7300fdn_driver", "build", "scanner", name),
		filepath.Join(home, "下载", "pantum", "m7300fdn_driver", "build2", "scanner", name),
		filepath.Join(home, "Downloads", "pantum", "m7300fdn_driver", "build", "scanner", name),
	}
	if p, err := exec.LookPath(name); err == nil {
		candidates[0] = p
	}
	best, bestTime := "", int64(-1)
	for _, c := range candidates {
		if c == "" {
			continue
		}
		st, err := os.Stat(c)
		if err != nil || st.IsDir() {
			continue
		}
		if st.ModTime().Unix() > bestTime {
			best, bestTime = c, st.ModTime().Unix()
		}
	}
	return best
}
