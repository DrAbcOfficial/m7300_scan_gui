<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  AddDevices,
  BrowseDirectory,
  CancelScan,
  FindBinary,
  LoadSettings,
  OpenFile,
  OpenFolder,
  RemoveDevice,
  RenameDevice,
  SaveSettings,
  ScanDevices,
  SetActiveDevice,
  StartScan,
} from '../wailsjs/go/backend/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type { backend } from '../wailsjs/go/models'
import { setLocale, currentLocale, SUPPORTED_LOCALES } from './i18n'

type Settings = backend.Settings
type Device = backend.Device
interface ScanProgress { page: number; info: string }
interface ScanResult { ok: boolean; pages: number; files: string[]; error: string; command: string }

const { t } = useI18n()

const LOCALE_LABEL: Record<string, string> = {
  'zh-CN': '中文',
  'en-US': 'English',
}

const settings = reactive({
  model: 'auto',
  host: '',
  devices: [] as Device[],
  activeHost: '',
  source: 'platen',
  resolution: 300,
  mode: 'color',
  regionFull: true,
  tlX: 0,
  tlY: 0,
  brX: 210,
  brY: 297,
  brightness: 0,
  contrast: 0,
  threshold: 128,
  format: 'png',
  quality: 90,
  maxPages: 500,
  outputDir: '',
  outputBase: 'scan',
  verbose: true,
  language: currentLocale(),
} as Settings)

const devices = ref<Device[]>([])
const activeHost = ref('')
const statusMsg = ref('')
const running = ref(false)
const files = ref<string[]>([])
const lastPages = ref(0)
const binaryMissing = ref(false)

const showAddDialog = ref(false)
const addScanning = ref(false)
const discovered = ref<Device[]>([])
const selectedHosts = ref<string[]>([])
const addDone = ref(false)

const showRenameDialog = ref(false)
const renameName = ref('')

const activeDevice = computed(() => devices.value.find(d => d.host === activeHost.value) || null)
const thresholdEnabled = computed(() => settings.mode === 'lineart')
const qualityEnabled = computed(() => settings.format !== 'png')

const EXT_BY_FORMAT: Record<string, string> = {
  png: 'png',
  jpg: 'jpg',
  pdf: 'pdf',
  'pdf-page': 'pdf',
}

const outputExt = computed(() => EXT_BY_FORMAT[settings.format] || 'png')

function rangeFill(value: number, min: number, max: number) {
  const pct = ((value - min) / (max - min)) * 100
  return { '--fill': `${pct}%` } as Record<string, string>
}

function fileName(path: string) {
  return path.replace(/^.*[/\\]/, '')
}

function persist() {
  settings.devices = devices.value
  settings.activeHost = activeHost.value
  SaveSettings({ ...settings })
}

function onBaseNameChange() {
  settings.outputBase = settings.outputBase.replace(/\.(png|jpe?g|pdf)$/i, '')
  persist()
}

function applySettings(s: Settings) {
  Object.assign(settings, s)
  devices.value = s.devices || []
  setLocale(settings.language)
  activeHost.value = s.activeHost || (devices.value.length ? devices.value[0].host : '')
  selectActive()
}

function selectActive() {
  const dev = devices.value.find(d => d.host === activeHost.value)
  if (dev) {
    settings.host = dev.host
    settings.model = dev.model
    persist()
    statusMsg.value = t('device.selected', { name: dev.name, host: dev.host })
    refreshBinary()
  } else {
    settings.host = ''
    settings.model = 'auto'
    statusMsg.value = t('status.ready')
  }
}

function onSelectDevice(host: string) {
  activeHost.value = host
  SetActiveDevice(host)
  selectActive()
}

function onDeviceChange(e: Event) {
  onSelectDevice((e.target as HTMLSelectElement).value)
}

async function refreshBinary() {
  const model = settings.model && settings.model !== 'auto' ? settings.model : ''
  binaryMissing.value = model ? !(await FindBinary(model)) : false
}

async function openAddDialog() {
  showAddDialog.value = true
  addDone.value = false
  await scanNow()
}

async function closeAddDialog() {
  showAddDialog.value = false
}

async function scanNow() {
  addScanning.value = true
  selectedHosts.value = []
  discovered.value = []
  addDone.value = false
  try {
    discovered.value = await ScanDevices()
  } finally {
    addScanning.value = false
    addDone.value = true
  }
}

async function confirmAdd() {
  const sel = discovered.value.filter(d => selectedHosts.value.includes(d.host))
  if (!sel.length) return
  const updated = await AddDevices(sel)
  devices.value = updated
  activeHost.value = sel[sel.length - 1].host
  SetActiveDevice(activeHost.value)
  selectActive()
  closeAddDialog()
}

function openRenameDialog() {
  if (!activeDevice.value) return
  renameName.value = activeDevice.value.name
  showRenameDialog.value = true
}

async function confirmRename() {
  const name = renameName.value.trim()
  if (!name || !activeDevice.value) {
    showRenameDialog.value = false
    return
  }
  devices.value = await RenameDevice(activeDevice.value.host, name)
  showRenameDialog.value = false
  statusMsg.value = t('device.selected', { name, host: activeDevice.value!.host })
}

async function removeActiveDevice() {
  const dev = activeDevice.value
  if (!dev) return
  const updated = await RemoveDevice(dev.host)
  devices.value = updated
  activeHost.value = ''
  SetActiveDevice('')
  selectActive()
}

async function browseDir() {
  const dir = await BrowseDirectory(settings.outputDir)
  if (dir) settings.outputDir = dir
}

async function startScan() {
  if (running.value) return
  if (binaryMissing.value) {
    statusMsg.value = `${t('device.binaryMissing', { cmd: settings.model + '-scan' })} — ${t('device.binaryMissingHint')}`
    return
  }
  persist()
  try {
    await StartScan({ ...settings })
  } catch (e) {
    statusMsg.value = String(e)
  }
}

function mergeFiles(newFiles: string[]) {
  const seen = new Set(files.value)
  const fresh = newFiles.filter(f => !seen.has(f))
  files.value = [...fresh, ...files.value]
}

function cancelScan() {
  CancelScan()
}

async function openResultFolder() {
  const dir = files.value.length ? files.value[0].replace(/[/\\][^/\\]+$/, '') : settings.outputDir
  await OpenFolder(dir)
}

function onLangChange() {
  setLocale(settings.language)
  persist()
}

onMounted(async () => {
  const s: Settings = await LoadSettings()
  applySettings(s)

  EventsOn('scan:start', () => {
    running.value = true
    statusMsg.value = t('action.scanning')
  })
  EventsOn('scan:progress', (p: ScanProgress) => {
    lastPages.value = p.page
    statusMsg.value = t('output.pages', { n: p.page })
  })
  EventsOn('scan:done', (r: ScanResult) => {
    running.value = false
    if (r.ok) {
      mergeFiles(r.files || [])
      statusMsg.value = t('output.pages', { n: r.pages || (r.files || []).length })
    } else {
      statusMsg.value = r.error === 'canceled' ? t('status.canceled') : `${t('result.failed')}: ${r.error}`
    }
  })
})

onBeforeUnmount(() => {
  if (running.value) CancelScan()
})
</script>

<template>
  <div class="app">
    <header class="top">
      <div class="brand">
        <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <rect x="3" y="7" width="18" height="10" rx="2" stroke="currentColor" stroke-width="1.6"/>
          <path d="M7 7V5.5A1.5 1.5 0 0 1 8.5 4h7A1.5 1.5 0 0 1 17 5.5V7M7 17v1.5A1.5 1.5 0 0 0 8.5 20h7a1.5 1.5 0 0 0 1.5-1.5V17" stroke="currentColor" stroke-width="1.6"/>
          <circle cx="17" cy="12" r="1.1" fill="currentColor"/>
        </svg>
        <span>{{ t('app.title') }}</span>
      </div>

      <div class="devbar">
        <select
          class="devpick"
          :value="activeHost"
          :disabled="!devices.length"
          @change="onDeviceChange"
        >
          <option v-if="!devices.length" value="">{{ t('device.choose') }}</option>
          <option v-for="d in devices" :key="d.host" :value="d.host">{{ d.name }}  ·  {{ d.host }}</option>
        </select>
        <button type="button" class="btn" @click="openAddDialog">{{ t('device.addDevice') }}</button>
        <button type="button" class="btn" :disabled="!activeDevice" @click="openRenameDialog">{{ t('device.rename') }}</button>
        <button type="button" class="btn danger" :disabled="!activeDevice" @click="removeActiveDevice">{{ t('device.delete') }}</button>
      </div>

      <select class="lang" v-model="settings.language" @change="onLangChange">
        <option v-for="l in SUPPORTED_LOCALES" :key="l" :value="l">{{ LOCALE_LABEL[l] || l }}</option>
      </select>
    </header>

    <div class="status" :class="{ warn: binaryMissing, run: running }">
      <span class="pulse" v-if="running"></span>
      <span class="stext">
        {{ binaryMissing
          ? `${t('device.binaryMissing', { cmd: settings.model + '-scan' })} — ${t('device.binaryMissingHint')}`
          : (statusMsg || t('status.ready')) }}
      </span>
    </div>

    <section class="scan" aria-label="scan">
      <h2>{{ t('scan.section') }}</h2>

      <div class="trio">
        <label class="field">
          <span>{{ t('scan.source') }}</span>
          <select v-model="settings.source" @change="persist">
            <option value="platen">{{ t('scan.sourcePlaten') }}</option>
            <option value="adf">{{ t('scan.sourceAdf') }}</option>
            <option value="adf-duplex">{{ t('scan.sourceAdfDuplex') }}</option>
          </select>
        </label>
        <label class="field">
          <span>{{ t('scan.resolution') }}</span>
          <select v-model.number="settings.resolution" @change="persist">
            <option :value="75">75 dpi</option>
            <option :value="150">150 dpi</option>
            <option :value="300">300 dpi</option>
          </select>
        </label>
        <label class="field">
          <span>{{ t('scan.mode') }}</span>
          <select v-model="settings.mode" @change="persist">
            <option value="color">{{ t('scan.modeColor') }}</option>
            <option value="gray">{{ t('scan.modeGray') }}</option>
            <option value="lineart">{{ t('scan.modeLineart') }}</option>
          </select>
        </label>
      </div>

      <div class="sliders">
        <label class="slider">
          <span>{{ t('scan.brightness') }}</span>
          <input v-model.number="settings.brightness" type="range" min="-100" max="100" :style="rangeFill(settings.brightness, -100, 100)" @input="persist" />
          <input v-model.number="settings.brightness" class="num" type="number" min="-100" max="100" @change="persist" />
        </label>
        <label class="slider">
          <span>{{ t('scan.contrast') }}</span>
          <input v-model.number="settings.contrast" type="range" min="-100" max="100" :style="rangeFill(settings.contrast, -100, 100)" @input="persist" />
          <input v-model.number="settings.contrast" class="num" type="number" min="-100" max="100" @change="persist" />
        </label>
        <label class="slider" :class="{ dim: !thresholdEnabled }">
          <span>{{ t('scan.threshold') }}</span>
          <input v-model.number="settings.threshold" type="range" min="0" max="255" :disabled="!thresholdEnabled" :style="rangeFill(settings.threshold, 0, 255)" @input="persist" />
          <input v-model.number="settings.threshold" class="num" type="number" min="0" max="255" :disabled="!thresholdEnabled" @change="persist" />
        </label>
      </div>

      <div class="region">
        <span class="rlab">{{ t('scan.region') }}</span>
        <label class="opt"><input v-model="settings.regionFull" type="radio" :value="true" @change="persist" /><span>{{ t('scan.regionFull') }}</span></label>
        <label class="opt"><input v-model="settings.regionFull" type="radio" :value="false" @change="persist" /><span>{{ t('scan.regionCustom') }}</span></label>
      </div>
      <div class="xy" :class="{ off: settings.regionFull }">
        <span>{{ t('scan.tl') }}</span>
        <input v-model.number="settings.tlX" class="num" type="number" min="0" @change="persist" />
        <input v-model.number="settings.tlY" class="num" type="number" min="0" @change="persist" />
        <span>{{ t('scan.br') }}</span>
        <input v-model.number="settings.brX" class="num" type="number" min="1" @change="persist" />
        <input v-model.number="settings.brY" class="num" type="number" min="1" @change="persist" />
        <span class="unit">mm</span>
      </div>
    </section>

    <section class="out" aria-label="output">
      <h2>{{ t('output.section') }}</h2>

      <div class="duo">
        <label class="field">
          <span>{{ t('output.format') }}</span>
          <select v-model="settings.format" @change="persist">
            <option value="png">{{ t('output.formatPng') }}</option>
            <option value="jpg">{{ t('output.formatJpg') }}</option>
            <option value="pdf">{{ t('output.formatPdf') }}</option>
            <option value="pdf-page">{{ t('output.formatPdfPage') }}</option>
          </select>
        </label>
        <label class="field">
          <span>{{ t('output.maxPages') }}</span>
          <input v-model.number="settings.maxPages" type="number" min="1" max="5000" @change="persist" />
        </label>
      </div>

      <label class="slider" :class="{ dim: !qualityEnabled }">
        <span>{{ t('output.quality') }}</span>
        <input v-model.number="settings.quality" type="range" min="0" max="100" :disabled="!qualityEnabled" :style="rangeFill(settings.quality, 0, 100)" @input="persist" />
        <span class="qval">{{ settings.quality }}</span>
      </label>

      <label class="field">
        <span>{{ t('output.base') }}</span>
        <div class="combo">
          <input v-model="settings.outputBase" type="text" @change="onBaseNameChange" />
          <em>.{{ outputExt }}</em>
        </div>
      </label>

      <label class="field">
        <span>{{ t('output.dir') }}</span>
        <div class="combo">
          <input v-model="settings.outputDir" type="text" @change="persist" />
          <button type="button" class="btn" @click="browseDir">{{ t('output.browse') }}</button>
        </div>
      </label>

      <label class="opt verbose">
        <input v-model="settings.verbose" type="checkbox" @change="persist" />
        <span>{{ t('output.verbose') }}</span>
      </label>

      <div class="go">
        <button type="button" class="btn scanbtn" :disabled="running" @click="startScan">
          {{ running ? t('action.scanning') : t('action.scan') }}
        </button>
        <button type="button" class="btn" :disabled="!running" @click="cancelScan">{{ t('action.cancel') }}</button>
      </div>
    </section>

    <section class="files" aria-label="results">
      <div class="files-h">
        <h2>{{ t('result.title') }}</h2>
        <span class="count" v-if="files.length">{{ files.length }}</span>
        <button type="button" class="btn" :disabled="!files.length" @click="openResultFolder">{{ t('action.openFolder') }}</button>
      </div>
      <ul>
        <li v-for="f in files" :key="f">
          <div class="fmeta" :title="f">
            <b>{{ fileName(f) }}</b>
            <small>{{ f }}</small>
          </div>
          <button type="button" class="btn" @click="OpenFile(f)">{{ t('action.openFile') }}</button>
        </li>
        <li v-if="!files.length" class="empty">{{ t('result.noFiles') }}</li>
      </ul>
    </section>

    <div v-if="showAddDialog" class="overlay" @click.self="closeAddDialog">
      <div class="modal">
        <div class="mhead">
          <h2>{{ t('device.addDialogTitle') }}</h2>
          <button type="button" class="btn icon" @click="closeAddDialog" :aria-label="t('device.close')">×</button>
        </div>
        <div class="mbody">
          <div class="mbar">
            <button type="button" class="btn" :disabled="addScanning" @click="scanNow">{{ t('device.scanNow') }}</button>
            <span v-if="addScanning" class="hint live">{{ t('device.scanning') }}</span>
            <span v-else-if="addDone" class="hint">
              {{ discovered.length ? t('device.found', { n: discovered.length }) : t('device.noneFound') }}
            </span>
          </div>
          <ul class="found">
            <li v-for="d in discovered" :key="d.host">
              <label>
                <input type="checkbox" :value="d.host" v-model="selectedHosts" />
                <b>{{ d.name }}</b>
                <em>{{ d.model }}</em>
                <code>{{ d.host }}</code>
              </label>
            </li>
          </ul>
        </div>
        <div class="mfoot">
          <button type="button" class="btn" @click="closeAddDialog">{{ t('device.close') }}</button>
          <button type="button" class="btn scanbtn sm" :disabled="!selectedHosts.length" @click="confirmAdd">
            {{ t('device.addSelected', { n: selectedHosts.length }) }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showRenameDialog" class="overlay" @click.self="showRenameDialog = false">
      <div class="modal narrow">
        <div class="mhead">
          <h2>{{ t('device.renameDialogTitle') }}</h2>
          <button type="button" class="btn icon" @click="showRenameDialog = false" :aria-label="t('device.cancel')">×</button>
        </div>
        <div class="mbody">
          <label class="field">
            <span>{{ t('device.deviceName') }}</span>
            <input v-model="renameName" type="text" @keyup.enter="confirmRename" />
          </label>
        </div>
        <div class="mfoot">
          <button type="button" class="btn" @click="showRenameDialog = false">{{ t('device.cancel') }}</button>
          <button type="button" class="btn scanbtn sm" @click="confirmRename">{{ t('device.save') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.app {
  height: 100%;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  grid-template-rows: 40px 26px minmax(0, 1fr) 148px;
  grid-template-areas:
    "top top"
    "status status"
    "scan out"
    "files files";
  background: var(--bg);
}

.top {
  grid-area: top;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  padding: 0 10px 0 12px;
  background: var(--header);
  color: var(--header-fg);
  border-bottom: 1px solid var(--header-line);
  min-width: 0;
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 650;
  font-size: 13px;
  letter-spacing: .2px;
  white-space: nowrap;
}
.brand svg { width: 18px; height: 18px; flex: none; color: var(--accent); }

.devbar {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.devpick {
  flex: 1;
  min-width: 0;
  max-width: 420px;
  background: #1b252c;
  color: var(--header-fg);
  border-color: #3a4750;
}
.devpick:hover:not(:disabled) { background: #223038; }
.lang {
  width: 92px;
  background: #1b252c;
  color: var(--header-fg);
  border-color: #3a4750;
}

.status {
  grid-area: status;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  background: var(--bg-2);
  border-bottom: 1px solid var(--line);
  color: var(--muted);
  font-size: 12px;
  min-width: 0;
}
.status.warn {
  background: var(--warn-bg);
  border-bottom-color: var(--warn-line);
  color: var(--warn-text);
}
.stext {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  user-select: text;
}
.pulse {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent);
  flex: none;
  animation: blink 1s ease-in-out infinite;
}
@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: .3; }
}

.scan, .out, .files {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.scan {
  grid-area: scan;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px 8px;
  border-right: 1px solid var(--line);
  background: var(--surface);
  overflow-y: auto;
}

.out {
  grid-area: out;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px 10px;
  background: var(--surface);
  overflow-y: auto;
}

.files {
  grid-area: files;
  display: flex;
  flex-direction: column;
  background: var(--bg-2);
  border-top: 1px solid var(--line);
}

h2 {
  margin: 0;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: .06em;
  text-transform: uppercase;
  color: var(--faint);
}

.trio, .duo {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.trio { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.duo { grid-template-columns: minmax(0, 1.4fr) minmax(0, .9fr); }

.field {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}
.field > span, .slider > span, .rlab {
  font-size: 11px;
  color: var(--muted);
  white-space: nowrap;
}

.sliders {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.slider {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr) 56px;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.slider.dim, .dim { opacity: .42; }
.qval {
  width: 32px;
  text-align: right;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.region {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 22px;
}
.xy {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  color: var(--muted);
  font-size: 12px;
}
.xy.off { visibility: hidden; }
.unit { color: var(--faint); }

.opt {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--text);
  font-size: 12.5px;
  cursor: pointer;
  white-space: nowrap;
}
.verbose { margin-top: 2px; }

.combo {
  display: flex;
  align-items: stretch;
  min-width: 0;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius);
  background: var(--control);
  overflow: hidden;
}
.combo input {
  flex: 1;
  min-width: 0;
  border: none;
  border-radius: 0;
}
.combo em {
  display: flex;
  align-items: center;
  padding: 0 8px;
  color: var(--faint);
  font-style: normal;
  font-size: 12px;
  background: var(--bg-2);
  border-left: 1px solid var(--line);
  flex: none;
}
.combo .btn {
  border: none;
  border-radius: 0;
  border-left: 1px solid var(--line);
}

.go {
  margin-top: auto;
  display: grid;
  grid-template-columns: 1fr 72px;
  gap: 6px;
  flex: none;
  position: sticky;
  bottom: 0;
  padding-top: 4px;
  background: var(--surface);
}

.files-h {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px 4px;
  flex: none;
}
.files-h h2 { margin-right: auto; }
.count {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9px;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.files ul {
  list-style: none;
  margin: 0;
  padding: 0 8px 8px;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}
.files li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: var(--radius);
  min-width: 0;
}
.files li:hover { background: var(--surface); }
.fmeta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.fmeta b {
  font-weight: 600;
  font-size: 12.5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fmeta small {
  color: var(--faint);
  font-size: 11px;
  font-family: Consolas, Menlo, monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  user-select: text;
}
.empty { color: var(--faint); font-size: 12.5px; padding: 10px 8px !important; }

input, select, button {
  font: inherit;
  color: var(--text);
}
input, select {
  width: 100%;
  height: 28px;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius);
  padding: 0 8px;
  background: var(--control);
  min-width: 0;
}
select {
  appearance: none;
  -webkit-appearance: none;
  padding-right: 22px;
  background-image:
    linear-gradient(45deg, transparent 50%, var(--muted) 50%),
    linear-gradient(135deg, var(--muted) 50%, transparent 50%);
  background-position: calc(100% - 12px) 50%, calc(100% - 7px) 50%;
  background-size: 5px 5px;
  background-repeat: no-repeat;
  cursor: pointer;
}
input:hover:not(:disabled), select:hover:not(:disabled) { background: var(--control-hover); }
input:focus-visible, select:focus-visible {
  outline: none;
  box-shadow: var(--focus);
  border-color: var(--accent);
}
input:disabled, select:disabled { opacity: .5; }
.num { width: 56px; padding: 0 4px; text-align: center; }

input[type="range"] {
  -webkit-appearance: none;
  appearance: none;
  height: 20px;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
}
input[type="range"]:hover:not(:disabled) { background: transparent; }
input[type="range"]::-webkit-slider-runnable-track {
  height: 4px;
  border-radius: 2px;
  background: linear-gradient(to right, var(--accent) var(--fill, 50%), var(--line-strong) var(--fill, 50%));
}
input[type="range"]::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 14px;
  height: 14px;
  margin-top: -5px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid var(--surface);
}
input[type="range"]:focus-visible { box-shadow: none; outline: 1px dotted var(--muted); }

input[type="checkbox"], input[type="radio"] {
  appearance: none;
  -webkit-appearance: none;
  width: 15px;
  height: 15px;
  margin: 0;
  padding: 0;
  flex: none;
  border: 1px solid var(--line-strong);
  background: var(--control);
  cursor: pointer;
}
input[type="checkbox"] { border-radius: 3px; }
input[type="radio"] { border-radius: 50%; }
input[type="checkbox"]:checked, input[type="radio"]:checked {
  background: var(--accent);
  border-color: var(--accent);
}
input[type="checkbox"]:checked {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 12 12'%3E%3Cpath d='M2.5 6.2l2.3 2.3 4.7-5' fill='none' stroke='white' stroke-width='1.8' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E");
  background-size: 11px;
  background-position: center;
  background-repeat: no-repeat;
}
input[type="radio"]:checked {
  background-image: radial-gradient(circle, var(--accent-fg) 0 3px, transparent 3.5px);
}

.btn {
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius);
  background: var(--control);
  color: var(--text);
  cursor: pointer;
  font-size: 12.5px;
  white-space: nowrap;
  flex: none;
}
.top .btn {
  background: #2e3a43;
  color: var(--header-fg);
  border-color: #45545e;
}
.top .btn:hover:not(:disabled) { background: #3a4852; }
.top .btn.danger { color: #ffb4ab; }
.btn:hover:not(:disabled) { background: var(--control-hover); }
.btn:disabled { opacity: .4; cursor: default; }
.btn:focus-visible { outline: none; box-shadow: var(--focus); }
.btn.danger { color: var(--danger); }
.scanbtn {
  background: var(--accent);
  border-color: transparent;
  color: var(--accent-fg);
  font-weight: 700;
  height: 36px;
  font-size: 14px;
}
.scanbtn.sm { height: 28px; font-size: 12.5px; }
.scanbtn:hover:not(:disabled) { background: var(--accent-hover); }
.scanbtn:active:not(:disabled) { background: var(--accent-press); }
.btn.icon {
  width: 28px;
  padding: 0;
  font-size: 18px;
  line-height: 1;
  background: transparent;
  border: none;
  color: var(--muted);
}

.overlay {
  position: fixed;
  inset: 0;
  background: rgba(20, 24, 28, .45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 20;
}
.modal {
  width: 520px;
  max-width: calc(100vw - 32px);
  max-height: calc(100vh - 32px);
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.modal.narrow { width: 360px; }
.mhead, .mfoot {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  flex: none;
}
.mhead { justify-content: space-between; border-bottom: 1px solid var(--line); }
.mhead h2 { font-size: 14px; letter-spacing: 0; text-transform: none; color: var(--text); }
.mfoot { justify-content: flex-end; border-top: 1px solid var(--line); background: var(--bg-2); }
.mbody { padding: 12px; overflow-y: auto; min-height: 0; }
.mbar { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.hint { color: var(--muted); font-size: 12.5px; }
.hint.live { color: var(--accent); }
.found { list-style: none; margin: 0; padding: 0; }
.found li { border-radius: var(--radius); }
.found li:hover { background: var(--bg-2); }
.found label {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  cursor: pointer;
  min-width: 0;
}
.found b {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}
.found em {
  font-style: normal;
  font-size: 11px;
  color: var(--accent);
  background: var(--accent-soft);
  border-radius: 999px;
  padding: 1px 8px;
  white-space: nowrap;
}
.found code {
  color: var(--faint);
  font-size: 12px;
  white-space: nowrap;
}
</style>
