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

const EXT_BY_FORMAT: Record<string, string> = {
  png: 'png',
  jpg: 'jpg',
  pdf: 'pdf',
  'pdf-page': 'pdf',
}

// 文件名自动按输出格式添加后缀
const outputFileName = computed(() => {
  const base = settings.outputBase || 'scan'
  return `${base}.${EXT_BY_FORMAT[settings.format] || 'png'}`
})

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

function onSelectDevice() {
  SetActiveDevice(activeHost.value)
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
  files.value = []
  lastPages.value = 0
  persist()
  try {
    await StartScan({ ...settings })
  } catch (e) {
    statusMsg.value = String(e)
  }
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
      files.value = r.files || []
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
    <main class="grid">
      <section class="card">
        <h2>{{ t('device.section') }}</h2>
        <select class="devlist" size="5" v-model="activeHost" @change="onSelectDevice">
          <option v-for="d in devices" :key="d.host" :value="d.host">{{ d.name }} ({{ d.host }})</option>
        </select>
        <div v-if="!devices.length" class="muted emptyhint">{{ t('device.noDevices') }}</div>
        <div class="devbtns">
          <button class="btn" @click="openAddDialog">{{ t('device.addDevice') }}</button>
          <button class="btn" :disabled="!activeDevice" @click="openRenameDialog">{{ t('device.rename') }}</button>
          <button class="btn danger" :disabled="!activeDevice" @click="removeActiveDevice">{{ t('device.delete') }}</button>
          <label class="lang">
            {{ t('language') }}
            <select v-model="settings.language" @change="onLangChange">
              <option v-for="l in SUPPORTED_LOCALES" :key="l" :value="l">{{ l }}</option>
            </select>
          </label>
        </div>
        <div class="status"><span>{{ statusMsg }}</span></div>
        <div v-if="binaryMissing" class="warn">
          {{ t('device.binaryMissing', { cmd: settings.model + '-scan' }) }} — {{ t('device.binaryMissingHint') }}
        </div>
      </section>

      <section class="card">
        <h2>{{ t('scan.section') }}</h2>
        <div class="row">
          <label>{{ t('scan.source') }}</label>
          <select v-model="settings.source" @change="persist">
            <option value="platen">{{ t('scan.sourcePlaten') }}</option>
            <option value="adf">{{ t('scan.sourceAdf') }}</option>
            <option value="adf-duplex">{{ t('scan.sourceAdfDuplex') }}</option>
          </select>
          <label class="inline">{{ t('scan.resolution') }}</label>
          <select v-model.number="settings.resolution" @change="persist">
            <option :value="75">75</option>
            <option :value="150">150</option>
            <option :value="300">300</option>
          </select>
        </div>
        <div class="row">
          <label>{{ t('scan.mode') }}</label>
          <select v-model="settings.mode" @change="persist">
            <option value="color">{{ t('scan.modeColor') }}</option>
            <option value="gray">{{ t('scan.modeGray') }}</option>
            <option value="lineart">{{ t('scan.modeLineart') }}</option>
          </select>
          <label class="inline" :class="{ disabled: !thresholdEnabled }">{{ t('scan.threshold') }}</label>
          <input v-model.number="settings.threshold" type="range" min="0" max="255" :disabled="!thresholdEnabled" @input="persist" />
          <input v-model.number="settings.threshold" class="num" type="number" min="0" max="255" :disabled="!thresholdEnabled" @change="persist" />
        </div>
        <div class="row">
          <label>{{ t('scan.brightness') }}</label>
          <input v-model.number="settings.brightness" type="range" min="-100" max="100" @input="persist" />
          <input v-model.number="settings.brightness" class="num" type="number" min="-100" max="100" @change="persist" />
          <label class="inline">{{ t('scan.contrast') }}</label>
          <input v-model.number="settings.contrast" type="range" min="-100" max="100" @input="persist" />
          <input v-model.number="settings.contrast" class="num" type="number" min="-100" max="100" @change="persist" />
        </div>
        <div class="row region-row">
          <label>{{ t('scan.region') }}</label>
          <label class="radio">
            <input v-model="settings.regionFull" type="radio" :value="true" @change="persist" />
            {{ t('scan.regionFull') }}
          </label>
          <label class="radio">
            <input v-model="settings.regionFull" type="radio" :value="false" @change="persist" />
            {{ t('scan.regionCustom') }}
          </label>
          <template v-if="!settings.regionFull">
            <label class="region-label">{{ t('scan.tl') }}</label>
            <input v-model.number="settings.tlX" type="number" min="0" @change="persist" />
            <input v-model.number="settings.tlY" type="number" min="0" @change="persist" />
            <label class="region-label">{{ t('scan.br') }}</label>
            <input v-model.number="settings.brX" type="number" min="1" @change="persist" />
            <input v-model.number="settings.brY" type="number" min="1" @change="persist" />
            <span class="mm">mm</span>
          </template>
        </div>
      </section>
    </main>

    <main class="grid">
      <section class="card">
        <h2>{{ t('output.section') }}</h2>
        <div class="row">
          <label>{{ t('output.format') }}</label>
          <select v-model="settings.format" @change="persist">
            <option value="png">{{ t('output.formatPng') }}</option>
            <option value="jpg">{{ t('output.formatJpg') }}</option>
            <option value="pdf">{{ t('output.formatPdf') }}</option>
            <option value="pdf-page">{{ t('output.formatPdfPage') }}</option>
          </select>
          <label class="inline" :class="{ disabled: !qualityEnabled }">{{ t('output.quality') }}</label>
          <input v-model.number="settings.quality" type="range" min="0" max="100" :disabled="!qualityEnabled" @input="persist" />
          <span>{{ settings.quality }}</span>
        </div>
        <div class="row">
          <label>{{ t('output.maxPages') }}</label>
          <input v-model.number="settings.maxPages" class="num" type="number" min="1" max="5000" @change="persist" />
          <label class="inline">{{ t('output.base') }}</label>
          <input v-model="settings.outputBase" type="text" @change="onBaseNameChange" />
          <span class="ext">{{ outputFileName }}</span>
        </div>
        <div class="row">
          <label>{{ t('output.dir') }}</label>
          <input v-model="settings.outputDir" type="text" @change="persist" />
          <button class="btn" @click="browseDir">{{ t('output.browse') }}</button>
        </div>
        <div class="row actions-row">
          <label class="checkbox">
            <input v-model="settings.verbose" type="checkbox" @change="persist" />
            {{ t('output.verbose') }}
          </label>
          <button class="btn primary" :disabled="running" @click="startScan">
            {{ running ? t('action.scanning') : t('action.scan') }}
          </button>
          <button class="btn danger" :disabled="!running" @click="cancelScan">{{ t('action.cancel') }}</button>
          <progress v-if="running" class="progress" value="0" />
        </div>
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

    <!-- 添加设备: WSD 扫描 (独立窗口外观) -->
    <div v-if="showAddDialog" class="overlay" @click.self="closeAddDialog">
      <div class="modal">
        <div class="modalhead">
          <h2>{{ t('device.addDialogTitle') }}</h2>
          <button class="btn small" @click="closeAddDialog">✕</button>
        </div>
        <div class="modalbody">
          <div class="modalbar">
            <button class="btn" :disabled="addScanning" @click="scanNow">{{ t('device.scanNow') }}</button>
            <span v-if="addScanning" class="spinner">{{ t('device.scanning') }}</span>
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
          <button class="btn primary" :disabled="!selectedHosts.length" @click="confirmAdd">
            {{ t('device.addSelected', { n: selectedHosts.length }) }}
          </button>
          <button class="btn" @click="closeAddDialog">{{ t('device.close') }}</button>
        </div>
      </div>
    </div>

    <!-- 重命名设备 -->
    <div v-if="showRenameDialog" class="overlay" @click.self="showRenameDialog = false">
      <div class="modal small">
        <div class="modalhead">
          <h2>{{ t('device.renameDialogTitle') }}</h2>
          <button class="btn small" @click="showRenameDialog = false">✕</button>
        </div>
        <div class="modalbody">
          <label class="row">{{ t('device.deviceName') }}</label>
          <input v-model="renameName" class="rename-input" type="text" @keyup.enter="confirmRename" />
        </div>
        <div class="modalfoot">
          <button class="btn primary" @click="confirmRename">{{ t('device.save') }}</button>
          <button class="btn" @click="showRenameDialog = false">{{ t('device.cancel') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.app {
  padding: 10px 16px 20px;
  max-width: 1080px;
  margin: 0 auto;
}
h2 { margin: 0 0 6px; font-size: 14px; color: #3d4450; }
.lang { margin-left: auto; color: #4a515c; font-size: 13px; display: inline-flex; align-items: center; }
.lang select { margin-left: 6px; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 10px; }
.card {
  background: #fff; border: 1px solid #e3e6ea; border-radius: 8px;
  padding: 10px 14px; box-shadow: 0 1px 2px rgba(0,0,0,.04);
  display: flex; flex-direction: column;
}
.row { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; flex-wrap: wrap; }
.row label, .row label.inline { min-width: 64px; color: #4a515c; }
.row label.inline { margin-left: 10px; }
.row input[type="text"] { flex: 1; min-width: 120px; }
.row select { min-width: 120px; }
.row .num { width: 58px; }
input, select {
  border: 1px solid #d5dae1; border-radius: 6px; padding: 5px 8px;
  background: #fff; color: inherit; font: inherit;
}
input:focus, select:focus { outline: 2px solid #4a8fe0; outline-offset: 0; border-color: transparent; }
input[type="range"] { flex: 1; min-width: 110px; accent-color: #3d7fd4; }
.status { margin: 2px 0 6px; display: flex; gap: 8px; align-items: center; color: #4a515c; font-size: 13px; }
.chip {
  background: #e8f2ff; color: #1e5fb4; border-radius: 20px; padding: 2px 10px; font-size: 12px;
  white-space: nowrap;
}
.warn {
  background: #fff6e0; border: 1px solid #f0d48a; color: #8a6d1a;
  border-radius: 6px; padding: 8px 10px; font-size: 13px; margin-top: 6px;
}
.btn {
  border: 1px solid #c9d0d9; background: #f7f8fa; border-radius: 6px;
  padding: 4px 12px; cursor: pointer; font: inherit; color: #2c3138;
}
.btn:hover:not(:disabled) { background: #eef1f4; }
.btn:disabled { opacity: .55; cursor: not-allowed; }
.btn.primary { background: #2f7fd9; border-color: #2f7fd9; color: #fff; }
.btn.primary:hover:not(:disabled) { background: #256bb8; }
.btn.danger { background: #fff; border-color: #e0a4a4; color: #c0392b; }
.btn.small { padding: 3px 10px; font-size: 12px; }
.region-row { gap: 6px; }
.radio { display: inline-flex; align-items: center; gap: 4px; margin-right: 8px; font-size: 13px; }
.region-row input[type="number"] { width: 52px; }
.region-label { color: #6a7280; font-size: 12px; min-width: 0 !important; }
.mm { color: #9aa3af; font-size: 12px; }
.checkbox { display: flex; align-items: center; gap: 6px; color: #4a515c; font-size: 13px; }
.ext { background: #eef1f4; border-radius: 5px; padding: 2px 8px; color: #586173; font-size: 12.5px; white-space: nowrap; }
.actions-row { margin-top: auto; padding-top: 6px; }
.progress { flex: 1; height: 8px; accent-color: #2f7fd9; }
.loghead { display: flex; justify-content: space-between; align-items: center; }
.files { list-style: none; margin: 0; padding: 0; flex: 1; overflow-y: auto; max-height: 150px; }
.file { display: flex; justify-content: space-between; gap: 8px; align-items: center; padding: 4px 0; border-bottom: 1px solid #eef0f3; }
.fname { word-break: break-all; font-size: 12.5px; color: #3d4450; }
.muted { color: #9aa3af; }

/* 设备列表 */
.devlist { width: 100%; min-height: 84px; font: inherit; }
.devlist option { padding: 4px 8px; }
.emptyhint { font-size: 12.5px; margin: -4px 0 6px; }
.devbtns { display: flex; gap: 8px; margin-top: 8px; }

/* 模态窗口 (添加设备 / 重命名) */
.overlay {
  position: fixed; inset: 0; background: rgba(20, 26, 34, .45);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.modal {
  background: #fff; border-radius: 10px; width: 480px; max-width: 92vw;
  box-shadow: 0 10px 40px rgba(0,0,0,.25); overflow: hidden;
  display: flex; flex-direction: column; max-height: 82vh;
}
.modal.small { width: 380px; }
.modalhead {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; border-bottom: 1px solid #e8ebef; background: #fafbfc;
}
.modalhead h2 { margin: 0; }
.modalbody { padding: 14px 16px; overflow-y: auto; }
.modalfoot {
  display: flex; justify-content: flex-end; gap: 10px;
  padding: 12px 16px; border-top: 1px solid #e8ebef;
}
.modalbar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.spinner { color: #3d7fd4; }
.found { color: #4a515c; font-size: 13px; }
.discovered { list-style: none; margin: 0; padding: 0; }
.drow { padding: 8px 4px; border-bottom: 1px solid #eef0f3; }
.drow label { display: flex; align-items: center; gap: 10px; cursor: pointer; }
.drow input[type="checkbox"] { accent-color: #2f7fd9; }
.dname { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dip { color: #7a828e; font-size: 12.5px; font-family: Consolas, Menlo, monospace; white-space: nowrap; }
.rename-input { width: 100%; box-sizing: border-box; margin-top: 4px; }
.disabled { opacity: .5; }
</style>
