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

// ---- add-device dialog (WSD scan) ----
const showAddDialog = ref(false)
const addScanning = ref(false)
const discovered = ref<Device[]>([])
const selectedHosts = ref<string[]>([])
const addDone = ref(false)

// ---- rename dialog ----
const showRenameDialog = ref(false)
const renameName = ref('')

const activeDevice = computed(() => devices.value.find(d => d.host === activeHost.value) || null)

const thresholdEnabled = computed(() => settings.mode === 'lineart')
const qualityEnabled = computed(() => settings.format !== 'png')

function rangeFill(value: number, min: number, max: number) {
  const pct = ((value - min) / (max - min)) * 100
  return { '--fill': `${pct}%` } as Record<string, string>
}

function persist() {
  // devices / activeHost 由独立 ref 管理, 保存前同步到 settings
  settings.devices = devices.value
  settings.activeHost = activeHost.value
  SaveSettings({ ...settings })
}

function onBaseNameChange() {
  settings.outputBase = settings.outputBase.replace(/\.(png|jpe?g|pdf)$/i, '')
  persist()
}

// ---- device management ----

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

// ---- scan actions ----

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
  const dir = files.value.length ? files.value[0].replace(/\/[^/]+$/, '') : settings.outputDir
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
    <header class="titlebar">
      <div class="brand">
        <svg class="brand-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="M4 8V5a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v3" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
          <rect x="2" y="8" width="20" height="8" rx="2" stroke="currentColor" stroke-width="1.6"/>
          <rect x="5" y="14" width="14" height="7" rx="1.5" stroke="currentColor" stroke-width="1.6"/>
          <path d="M7 17.5h10" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
          <circle cx="18" cy="11" r="1" fill="currentColor"/>
        </svg>
        <h1>Pantum Scanner</h1>
      </div>
      <label class="lang">
        <span>{{ t('language') }}</span>
        <select v-model="settings.language" @change="onLangChange">
          <option v-for="l in SUPPORTED_LOCALES" :key="l" :value="l">{{ l }}</option>
        </select>
      </label>
    </header>

    <div class="infobar" :class="{ warn: binaryMissing }">
      <svg v-if="!binaryMissing" class="ib-icon" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
        <path d="M8 1a7 7 0 1 1 0 14A7 7 0 0 1 8 1Zm0 3.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5ZM7.25 7v4.5h1.5V7h-1.5Z"/>
      </svg>
      <svg v-else class="ib-icon" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
        <path d="M7.16 1.68a1 1 0 0 1 1.68 0l6 10.5A1 1 0 0 1 14 13.9H2a1 1 0 0 1-.84-1.72l6-10.5ZM8 6v3.5h1V6H8Zm0 5.25a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Z"/>
      </svg>
      <span class="ib-text">
        {{ binaryMissing
          ? `${t('device.binaryMissing', { cmd: settings.model + '-scan' })} — ${t('device.binaryMissingHint')}`
          : (statusMsg || t('status.ready')) }}
      </span>
    </div>

    <main class="grid">
      <section class="card">
        <h2>{{ t('device.section') }}</h2>
        <div class="devlist" role="listbox">
          <button
            v-for="d in devices"
            :key="d.host"
            type="button"
            class="devitem"
            :class="{ active: d.host === activeHost }"
            @click="onSelectDevice(d.host)"
          >
            <span class="devname">{{ d.name }}</span>
            <span class="devhost">{{ d.host }}</span>
          </button>
          <div v-if="!devices.length" class="muted emptyhint">{{ t('device.noDevices') }}</div>
        </div>
        <div class="devbtns">
          <button class="btn" @click="openAddDialog">{{ t('device.addDevice') }}</button>
          <button class="btn" :disabled="!activeDevice" @click="openRenameDialog">{{ t('device.rename') }}</button>
          <button class="btn danger" :disabled="!activeDevice" @click="removeActiveDevice">{{ t('device.delete') }}</button>
        </div>
      </section>

      <section class="card">
        <h2>{{ t('scan.section') }}</h2>
        <div class="formgrid">
          <label>{{ t('scan.source') }}</label>
          <select v-model="settings.source" @change="persist">
            <option value="platen">{{ t('scan.sourcePlaten') }}</option>
            <option value="adf">{{ t('scan.sourceAdf') }}</option>
            <option value="adf-duplex">{{ t('scan.sourceAdfDuplex') }}</option>
          </select>
          <label>{{ t('scan.resolution') }}</label>
          <select v-model.number="settings.resolution" @change="persist">
            <option :value="75">75 dpi</option>
            <option :value="150">150 dpi</option>
            <option :value="300">300 dpi</option>
          </select>

          <label>{{ t('scan.mode') }}</label>
          <select v-model="settings.mode" @change="persist">
            <option value="color">{{ t('scan.modeColor') }}</option>
            <option value="gray">{{ t('scan.modeGray') }}</option>
            <option value="lineart">{{ t('scan.modeLineart') }}</option>
          </select>
          <label :class="{ disabled: !thresholdEnabled }">{{ t('scan.threshold') }}</label>
          <div class="sliderpair">
            <input v-model.number="settings.threshold" type="range" min="0" max="255" :disabled="!thresholdEnabled" :style="rangeFill(settings.threshold, 0, 255)" @input="persist" />
            <input v-model.number="settings.threshold" class="num" type="number" min="0" max="255" :disabled="!thresholdEnabled" @change="persist" />
          </div>

          <label>{{ t('scan.brightness') }}</label>
          <div class="sliderpair">
            <input v-model.number="settings.brightness" type="range" min="-100" max="100" :style="rangeFill(settings.brightness, -100, 100)" @input="persist" />
            <input v-model.number="settings.brightness" class="num" type="number" min="-100" max="100" @change="persist" />
          </div>
          <label>{{ t('scan.contrast') }}</label>
          <div class="sliderpair">
            <input v-model.number="settings.contrast" type="range" min="-100" max="100" :style="rangeFill(settings.contrast, -100, 100)" @input="persist" />
            <input v-model.number="settings.contrast" class="num" type="number" min="-100" max="100" @change="persist" />
          </div>

          <label>{{ t('scan.region') }}</label>
          <div class="region">
            <label class="radio">
              <input v-model="settings.regionFull" type="radio" :value="true" @change="persist" />
              <span>{{ t('scan.regionFull') }}</span>
            </label>
            <label class="radio">
              <input v-model="settings.regionFull" type="radio" :value="false" @change="persist" />
              <span>{{ t('scan.regionCustom') }}</span>
            </label>
          </div>
          <template v-if="!settings.regionFull">
            <span></span>
            <div class="regionvals">
              <span class="region-label">{{ t('scan.tl') }}</span>
              <input v-model.number="settings.tlX" class="num" type="number" min="0" @change="persist" />
              <input v-model.number="settings.tlY" class="num" type="number" min="0" @change="persist" />
              <span class="region-label">{{ t('scan.br') }}</span>
              <input v-model.number="settings.brX" class="num" type="number" min="1" @change="persist" />
              <input v-model.number="settings.brY" class="num" type="number" min="1" @change="persist" />
              <span class="mm">mm</span>
            </div>
          </template>
        </div>
      </section>

      <section class="card">
        <h2>{{ t('output.section') }}</h2>
        <div class="formgrid">
          <label>{{ t('output.format') }}</label>
          <select v-model="settings.format" @change="persist">
            <option value="png">{{ t('output.formatPng') }}</option>
            <option value="jpg">{{ t('output.formatJpg') }}</option>
            <option value="pdf">{{ t('output.formatPdf') }}</option>
            <option value="pdf-page">{{ t('output.formatPdfPage') }}</option>
          </select>
          <label :class="{ disabled: !qualityEnabled }">{{ t('output.quality') }}</label>
          <div class="sliderpair">
            <input v-model.number="settings.quality" type="range" min="0" max="100" :disabled="!qualityEnabled" :style="rangeFill(settings.quality, 0, 100)" @input="persist" />
            <span class="sliderval">{{ settings.quality }}</span>
          </div>

          <label>{{ t('output.maxPages') }}</label>
          <input v-model.number="settings.maxPages" class="num" type="number" min="1" max="5000" @change="persist" />
          <label>{{ t('output.base') }}</label>
          <input v-model="settings.outputBase" type="text" @change="onBaseNameChange" />

          <label>{{ t('output.dir') }}</label>
          <div class="dirpair wide">
            <input v-model="settings.outputDir" type="text" @change="persist" />
            <button class="btn" @click="browseDir">{{ t('output.browse') }}</button>
          </div>

          <span></span>
          <div class="actions wide">
            <label class="checkbox">
              <input v-model="settings.verbose" type="checkbox" @change="persist" />
              <span>{{ t('output.verbose') }}</span>
            </label>
            <span class="spacer"></span>
            <button class="btn accent" :disabled="running" @click="startScan">
              {{ running ? t('action.scanning') : t('action.scan') }}
            </button>
            <button class="btn danger" :disabled="!running" @click="cancelScan">{{ t('action.cancel') }}</button>
          </div>
        </div>
        <div v-if="running" class="progressline"><span></span></div>
      </section>

      <section class="card">
        <div class="loghead">
          <h2>{{ t('result.title') }}</h2>
          <button class="btn small" :disabled="!files.length" @click="openResultFolder">{{ t('action.openFolder') }}</button>
        </div>
        <ul class="files">
          <li v-for="f in files" :key="f" class="file">
            <span class="fname">{{ f }}</span>
            <button class="btn small" @click="OpenFile(f)">{{ t('action.openFile') }}</button>
          </li>
          <li v-if="!files.length" class="muted">{{ t('result.noFiles') }}</li>
        </ul>
      </section>
    </main>

    <!-- 添加设备: WSD 扫描 -->
    <div v-if="showAddDialog" class="overlay" @click.self="closeAddDialog">
      <div class="modal">
        <div class="modalhead">
          <h2>{{ t('device.addDialogTitle') }}</h2>
          <button class="btn icon" @click="closeAddDialog" :aria-label="t('device.close')">
            <svg viewBox="0 0 12 12" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round">
              <path d="M2 2l8 8M10 2l-8 8"/>
            </svg>
          </button>
        </div>
        <div class="modalbody">
          <div class="modalbar">
            <button class="btn" :disabled="addScanning" @click="scanNow">{{ t('device.scanNow') }}</button>
            <span v-if="addScanning" class="scanning">
              <span class="dotspin"></span>{{ t('device.scanning') }}
            </span>
            <span v-else-if="addDone" class="found">
              {{ discovered.length ? t('device.found', { n: discovered.length }) : t('device.noneFound') }}
            </span>
          </div>
          <ul class="discovered">
            <li v-for="d in discovered" :key="d.host" class="drow">
              <label>
                <input type="checkbox" :value="d.host" v-model="selectedHosts" />
                <span class="dname">{{ d.name }}</span>
                <span class="chip">{{ d.model }}</span>
                <span class="dip">{{ d.host }}</span>
              </label>
            </li>
          </ul>
        </div>
        <div class="modalfoot">
          <button class="btn" @click="closeAddDialog">{{ t('device.close') }}</button>
          <button class="btn accent" :disabled="!selectedHosts.length" @click="confirmAdd">
            {{ t('device.addSelected', { n: selectedHosts.length }) }}
          </button>
        </div>
      </div>
    </div>

    <!-- 重命名设备 -->
    <div v-if="showRenameDialog" class="overlay" @click.self="showRenameDialog = false">
      <div class="modal small">
        <div class="modalhead">
          <h2>{{ t('device.renameDialogTitle') }}</h2>
          <button class="btn icon" @click="showRenameDialog = false" :aria-label="t('device.cancel')">
            <svg viewBox="0 0 12 12" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round">
              <path d="M2 2l8 8M10 2l-8 8"/>
            </svg>
          </button>
        </div>
        <div class="modalbody">
          <label class="field-label">{{ t('device.deviceName') }}</label>
          <input v-model="renameName" class="rename-input" type="text" @keyup.enter="confirmRename" />
        </div>
        <div class="modalfoot">
          <button class="btn" @click="showRenameDialog = false">{{ t('device.cancel') }}</button>
          <button class="btn accent" @click="confirmRename">{{ t('device.save') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.app {
  padding: 12px 16px 16px;
  max-width: 1060px;
  margin: 0 auto;
}

/* ---------- 标题栏 ---------- */
.titlebar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.brand { display: flex; align-items: center; gap: 8px; color: var(--fd-accent); }
.brand-icon { width: 20px; height: 20px; }
.brand h1 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--fd-text);
  letter-spacing: .2px;
}
.lang { display: inline-flex; align-items: center; gap: 6px; color: var(--fd-text-secondary); font-size: 12.5px; }

/* ---------- 状态栏 (InfoBar) ---------- */
.infobar {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--fd-card);
  border: 1px solid var(--fd-card-border);
  border-radius: var(--fd-radius-sm);
  padding: 7px 12px;
  margin-bottom: 10px;
  color: var(--fd-text-secondary);
  font-size: 12.5px;
  box-shadow: var(--fd-shadow-card);
  min-height: 18px;
}
.infobar.warn {
  background: var(--fd-warn-bg);
  border-color: var(--fd-warn-border);
  color: var(--fd-warn-text);
}
.ib-icon { width: 15px; height: 15px; flex: none; color: var(--fd-accent); }
.infobar.warn .ib-icon { color: var(--fd-warn-text); }
.ib-text { user-select: text; }

/* ---------- 布局 / 卡片 ---------- */
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 10px; }
.grid:last-of-type { margin-bottom: 0; }
.card {
  background: var(--fd-card);
  border: 1px solid var(--fd-card-border);
  border-radius: var(--fd-radius);
  padding: 12px 14px;
  box-shadow: var(--fd-shadow-card);
  display: flex;
  flex-direction: column;
  position: relative;
}
h2 {
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--fd-text);
}
.loghead { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.loghead h2 { margin: 0; }

/* ---------- 表单网格 ---------- */
.formgrid {
  display: grid;
  grid-template-columns: auto 1fr auto 1fr;
  gap: 8px 10px;
  align-items: center;
}
.formgrid > label { color: var(--fd-text-secondary); font-size: 12.5px; white-space: nowrap; }
.formgrid > label.disabled { opacity: .45; }
.wide { grid-column: span 3; }
.sliderpair { display: flex; align-items: center; gap: 8px; min-width: 0; }
.sliderpair input[type="range"] { flex: 1; min-width: 60px; }
.sliderval { min-width: 24px; text-align: right; color: var(--fd-text-secondary); font-size: 12.5px; }
.dirpair { display: flex; align-items: center; gap: 8px; min-width: 0; }
.dirpair input { flex: 1; min-width: 0; }
.actions { display: flex; align-items: center; gap: 8px; }
.actions .spacer { flex: 1; }
.region { display: flex; gap: 14px; grid-column: span 3; }
.regionvals { display: flex; align-items: center; gap: 6px; grid-column: span 3; flex-wrap: wrap; }
.region-label { color: var(--fd-text-tertiary); font-size: 12px; }
.mm { color: var(--fd-text-tertiary); font-size: 12px; }

/* ---------- 输入控件 ---------- */
input, select {
  border: 1px solid var(--fd-control-border);
  border-bottom-color: var(--fd-control-border-strong);
  border-radius: var(--fd-radius-sm);
  padding: 4px 8px;
  background: var(--fd-control-bg);
  color: var(--fd-text);
  font: inherit;
  font-size: 12.5px;
  transition: border-color .1s ease, box-shadow .1s ease;
}
input:hover:not(:disabled), select:hover:not(:disabled) { background: var(--fd-control-hover); }
input:focus-visible, select:focus-visible {
  outline: none;
  border-color: var(--fd-accent);
  border-bottom-color: var(--fd-accent);
  box-shadow: 0 1px 0 var(--fd-accent);
}
input:disabled, select:disabled { opacity: .45; }
.num { width: 56px; }
select {
  appearance: none;
  -webkit-appearance: none;
  padding-right: 24px;
  background-image: linear-gradient(45deg, transparent 50%, var(--fd-text-secondary) 50%),
    linear-gradient(135deg, var(--fd-text-secondary) 50%, transparent 50%);
  background-position: calc(100% - 13px) 50%, calc(100% - 8px) 50%;
  background-size: 5px 5px;
  background-repeat: no-repeat;
  cursor: pointer;
}

/* Fluent 滑块 */
input[type="range"] {
  -webkit-appearance: none;
  appearance: none;
  background: transparent;
  border: none;
  padding: 0;
  height: 20px;
  cursor: pointer;
}
input[type="range"]:hover:not(:disabled) { background: transparent; }
input[type="range"]::-webkit-slider-runnable-track {
  height: 4px;
  border-radius: 2px;
  background: linear-gradient(to right, var(--fd-accent) var(--fill, 50%), var(--fd-control-border-strong) var(--fill, 50%));
}
input[type="range"]::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 14px;
  height: 14px;
  margin-top: -5px;
  border-radius: 50%;
  background: var(--fd-accent);
  border: 3px solid var(--fd-control-bg);
  box-shadow: 0 0 0 1px var(--fd-control-border-strong);
  transition: transform .1s ease;
}
input[type="range"]:hover:not(:disabled)::-webkit-slider-thumb { transform: scale(1.15); }
input[type="range"]:active:not(:disabled)::-webkit-slider-thumb { transform: scale(.95); border-width: 2px; }
input[type="range"]:focus-visible { box-shadow: none; outline: 1px dotted var(--fd-text-tertiary); outline-offset: 2px; }

/* 复选 / 单选 */
.checkbox, .radio { display: inline-flex; align-items: center; gap: 6px; color: var(--fd-text-secondary); font-size: 12.5px; cursor: pointer; }
input[type="checkbox"], input[type="radio"] {
  appearance: none;
  -webkit-appearance: none;
  width: 16px;
  height: 16px;
  border: 1px solid var(--fd-control-border-strong);
  background: var(--fd-control-bg);
  padding: 0;
  margin: 0;
  cursor: pointer;
  flex: none;
}
input[type="checkbox"] { border-radius: 3px; }
input[type="radio"] { border-radius: 50%; }
input[type="checkbox"]:checked, input[type="radio"]:checked {
  background: var(--fd-accent);
  border-color: var(--fd-accent);
}
input[type="checkbox"]:checked {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 12 12'%3E%3Cpath d='M2.5 6.2l2.3 2.3 4.7-5' fill='none' stroke='white' stroke-width='1.8' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E");
  background-size: 11px;
  background-position: center;
  background-repeat: no-repeat;
}
input[type="radio"]:checked {
  background-image: radial-gradient(circle, #fff 0 3.5px, transparent 4px);
}
input[type="checkbox"]:hover:not(:checked), input[type="radio"]:hover:not(:checked) { background: var(--fd-control-hover); }

/* ---------- 按钮 ---------- */
.btn {
  border: 1px solid var(--fd-control-border);
  border-bottom-color: var(--fd-control-border-strong);
  background: var(--fd-control-bg);
  border-radius: var(--fd-radius-sm);
  padding: 4px 14px;
  cursor: pointer;
  font: inherit;
  font-size: 12.5px;
  color: var(--fd-text);
  transition: background .08s ease;
  white-space: nowrap;
}
.btn:hover:not(:disabled) { background: var(--fd-control-hover); }
.btn:active:not(:disabled) { background: var(--fd-control-pressed); }
.btn:disabled { opacity: .4; cursor: default; }
.btn:focus-visible { outline: 2px solid var(--fd-text); outline-offset: 1px; }
.btn.accent {
  background: var(--fd-accent);
  border-color: transparent;
  color: #fff;
  font-weight: 600;
}
.btn.accent:hover:not(:disabled) { background: var(--fd-accent-hover); }
.btn.accent:active:not(:disabled) { background: var(--fd-accent-pressed); }
.btn.accent:focus-visible { outline-color: var(--fd-text); }
.btn.danger { color: var(--fd-danger); }
.btn.danger:hover:not(:disabled) { background: var(--fd-danger-hover-bg); border-color: var(--fd-danger); }
.btn.danger:active:not(:disabled) { background: var(--fd-danger-pressed-bg); }
.btn.small { padding: 2px 10px; font-size: 12px; }
.btn.icon {
  padding: 4px;
  border: none;
  background: transparent;
  color: var(--fd-text-secondary);
  border-radius: var(--fd-radius-sm);
  display: inline-flex;
}
.btn.icon:hover:not(:disabled) { background: var(--fd-control-hover); }

/* ---------- 设备列表 ---------- */
.devlist {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-height: 88px;
  max-height: 120px;
  overflow-y: auto;
  border: 1px solid var(--fd-card-border);
  border-radius: var(--fd-radius-sm);
  background: var(--fd-bg-layer);
  padding: 3px;
}
.devitem {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 5px 10px;
  border: none;
  border-radius: var(--fd-radius-sm);
  background: transparent;
  color: var(--fd-text);
  font: inherit;
  font-size: 12.5px;
  cursor: pointer;
  text-align: left;
  position: relative;
}
.devitem:hover { background: var(--fd-control-hover); }
.devitem.active { background: var(--fd-control-hover); }
.devitem.active::before {
  content: "";
  position: absolute;
  left: 0;
  top: 20%;
  bottom: 20%;
  width: 3px;
  border-radius: 2px;
  background: var(--fd-accent);
}
.devname { font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.devhost { color: var(--fd-text-tertiary); font-size: 11.5px; font-family: Consolas, Menlo, monospace; }
.emptyhint { padding: 8px 10px; font-size: 12.5px; }
.devbtns { display: flex; gap: 8px; margin-top: 10px; }
.muted { color: var(--fd-text-tertiary); }

/* ---------- 输出 / 结果 ---------- */
.progressline {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 3px;
  border-radius: 0 0 var(--fd-radius) var(--fd-radius);
  overflow: hidden;
}
.progressline span {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 40%;
  border-radius: 2px;
  background: var(--fd-accent);
  animation: fd-indeterminate 1.6s cubic-bezier(.4, 0, .6, 1) infinite;
}
@keyframes fd-indeterminate {
  0% { left: -40%; }
  100% { left: 100%; }
}
.files { list-style: none; margin: 0; padding: 0; flex: 1; overflow-y: auto; max-height: 132px; min-height: 88px; }
.file {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
  padding: 5px 4px;
  border-radius: var(--fd-radius-sm);
}
.file:hover { background: var(--fd-control-hover); }
.fname { word-break: break-all; font-size: 12px; color: var(--fd-text-secondary); font-family: Consolas, Menlo, monospace; user-select: text; }

/* ---------- 弹窗 ---------- */
.overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, .35);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  animation: fd-fade .12s ease;
}
@keyframes fd-fade { from { opacity: 0; } }
.modal {
  background: var(--fd-card);
  border: 1px solid var(--fd-card-border);
  border-radius: var(--fd-radius);
  width: 500px;
  max-width: 92vw;
  box-shadow: var(--fd-shadow-flyout);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  max-height: 82vh;
  animation: fd-pop .15s cubic-bezier(.2, 1.2, .4, 1);
}
@keyframes fd-pop { from { opacity: 0; transform: scale(.97) translateY(6px); } }
.modal.small { width: 380px; }
.modalhead {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px 8px;
}
.modalhead h2 { margin: 0; font-size: 15px; }
.modalbody { padding: 8px 16px 14px; overflow-y: auto; }
.modalfoot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--fd-card-border);
  background: var(--fd-bg-layer);
}
.modalbar { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; }
.scanning { display: inline-flex; align-items: center; gap: 8px; color: var(--fd-accent); font-size: 12.5px; }
.dotspin {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  border: 2px solid var(--fd-accent-subtle);
  border-top-color: var(--fd-accent);
  animation: fd-spin .7s linear infinite;
}
@keyframes fd-spin { to { transform: rotate(360deg); } }
.found { color: var(--fd-text-secondary); font-size: 12.5px; }
.discovered { list-style: none; margin: 0; padding: 0; }
.drow { border-radius: var(--fd-radius-sm); }
.drow:hover { background: var(--fd-control-hover); }
.drow label { display: flex; align-items: center; gap: 10px; cursor: pointer; padding: 7px 8px; }
.dname { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.chip {
  background: var(--fd-accent-subtle);
  color: var(--fd-accent);
  border-radius: 999px;
  padding: 1px 9px;
  font-size: 11.5px;
  white-space: nowrap;
}
.dip { color: var(--fd-text-tertiary); font-size: 12px; font-family: Consolas, Menlo, monospace; white-space: nowrap; }
.field-label { display: block; color: var(--fd-text-secondary); font-size: 12.5px; margin-bottom: 6px; }
.rename-input { width: 100%; box-sizing: border-box; }

@media (max-width: 940px) {
  .grid { grid-template-columns: 1fr; }
  .formgrid { grid-template-columns: auto 1fr; }
  .wide, .region, .regionvals { grid-column: span 1; }
}
</style>
