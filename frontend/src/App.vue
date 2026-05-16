<script setup lang="ts">
import { computed, onMounted, reactive } from "vue";
import {
  AlertTriangle,
  Bell,
  Check,
  Clock3,
  Database,
  FileText,
  Gauge,
  Info,
  KeyRound,
  Layers3,
  LogOut,
  MessageSquare,
  PenLine,
  RadioTower,
  RefreshCw,
  Router,
  Search,
  Send,
  Settings2,
  TerminalSquare,
  Wifi,
  WifiOff,
  X
} from "lucide-vue-next";
import { api, ApiError } from "./api";
import type { Device, DeviceType, Message, ObservedTopic, Page, ScheduledPublishTask, SystemAlert, User } from "./types";

type DeviceAction = "overview" | "topics" | "messages" | "publish" | "schedule" | "capabilities" | "type";
type CapabilityCommandId =
  | "emit"
  | "erase"
  | "learn"
  | "learnBatch"
  | "learnCancel"
  | "reset"
  | "restart"
  | "info"
  | "read"
  | "write"
  | "mqtt"
  | "tcp"
  | "wifiLock";

type InfraredCapability = {
  id: CapabilityCommandId;
  title: string;
  commandName: string;
  description: string;
  needsNo?: boolean;
  needsFile?: boolean;
  needsIrData?: boolean;
  connection?: "mqtt" | "tcp";
  setting?: "wifiLock";
  danger?: boolean;
};

const GSCU1B_TYPE = "GSCU1B";
const infraredCapabilities: InfraredCapability[] = [
  {
    id: "emit",
    title: "发射学习过的红外码",
    commandName: "controller-infrared-emit",
    description: "按编号发射已经学习或写入的红外码。",
    needsNo: true
  },
  {
    id: "erase",
    title: "擦除红外码",
    commandName: "controller-infrared-erase",
    description: "擦除设备中已保存的红外码。"
  },
  {
    id: "learn",
    title: "学习红外码",
    commandName: "controller-infrared-learn",
    description: "让设备进入指定编号的红外学习流程。",
    needsNo: true
  },
  {
    id: "learnBatch",
    title: "批量写入红外码",
    commandName: "controller-infrared-learn-batch",
    description: "通过红外码文件下载地址批量写入码值。",
    needsFile: true
  },
  {
    id: "learnCancel",
    title: "取消学习红外码",
    commandName: "controller-infrared-learn-cancel",
    description: "取消当前红外学习流程。"
  },
  {
    id: "reset",
    title: "重置/恢复出厂设置",
    commandName: "controller-reset",
    description: "恢复出厂设置会清空配网信息和自定义信息。",
    danger: true
  },
  {
    id: "restart",
    title: "设备软重启",
    commandName: "controller-restart",
    description: "让设备重启，并让通过指令设置的参数生效。"
  },
  {
    id: "info",
    title: "获取设备信息",
    commandName: "info-all",
    description: "获取信号强度、MAC、固件版本、IP、SSID、配网锁等信息。"
  },
  {
    id: "read",
    title: "读取红外码",
    commandName: "ir_read",
    description: "读取指定编号的红外码数据。",
    needsNo: true
  },
  {
    id: "write",
    title: "写入红外码",
    commandName: "ir_write",
    description: "把 HEX 红外码数据写入指定编号。",
    needsNo: true,
    needsIrData: true
  },
  {
    id: "mqtt",
    title: "自定义MQTT",
    commandName: "setting-mqtt",
    description: "设置自定义 MQTT 服务器信息，重启后生效。",
    connection: "mqtt"
  },
  {
    id: "tcp",
    title: "自定义TCP",
    commandName: "setting-tcp",
    description: "设置自定义 TCP 服务器信息，重启后生效。",
    connection: "tcp"
  },
  {
    id: "wifiLock",
    title: "设置Wifi配网锁",
    commandName: "setting-wifi-lock",
    description: "开启后无法通过长按配网按钮进入配网，只能通过 MQTT/TCP 解锁。",
    setting: "wifiLock"
  }
];

const session = reactive({
  loading: true,
  user: null as User | null,
  loginError: "",
  username: "admin",
  password: ""
});

const state = reactive({
  loading: false,
  error: "",
  q: "",
  status: "",
  selectedDevice: null as Device | null,
  activeAction: null as DeviceAction | null,
  showAlerts: false,
  devices: emptyPage<Device>(),
  deviceTopics: emptyPage<ObservedTopic>(),
  messages: emptyPage<Message>(),
  scheduledTasks: emptyPage<ScheduledPublishTask>(),
  alerts: emptyPage<SystemAlert>(),
  deviceTypes: [] as DeviceType[]
});

const publishForm = reactive({
  topic: "",
  payload: "",
  qos: 0,
  retain: false,
  result: "",
  error: ""
});

const scheduleForm = reactive({
  name: "",
  topic: "",
  payload: "",
  qos: 0,
  retain: false,
  scheduleType: "once" as "once" | "daily" | "weekly",
  runAtLocal: "",
  timeOfDay: "09:00",
  weekdays: [1, 2, 3, 4, 5] as number[],
  result: "",
  error: ""
});

const capabilityForm = reactive({
  commandId: "emit" as CapabilityCommandId,
  topic: "",
  messageId: "",
  no: 110,
  fileUrl: "http://geek-smart-pub.oss-cn-qingdao.aliyuncs.com/new.bin",
  irData: "",
  wifiLock: 0,
  mqttServer: "192.168.0.66",
  mqttPort: "1883",
  mqttClientId: "admin",
  mqttUsername: "username",
  mqttPassword: "password",
  mqttPublish: "/topic/qos0",
  mqttSubscribe: "/topic/qos1",
  tcpServer: "192.168.0.66",
  tcpPort: "1883",
  qos: 0,
  retain: false,
  result: "",
  error: ""
});

const onlineCount = computed(() => state.devices.items.filter((item) => item.status === "online").length);
const offlineCount = computed(() => state.devices.items.filter((item) => item.status !== "online").length);
const alertCount = computed(() => state.alerts.items.filter((item) => item.status === "open").length);
const selectedCapability = computed(
  () => infraredCapabilities.find((item) => item.id === capabilityForm.commandId) ?? infraredCapabilities[0]
);
const capabilityPayloadPreview = computed(() => JSON.stringify(buildCapabilityPayload(), null, 2));
const actionTitle = computed(() => {
  if (!state.selectedDevice || !state.activeAction) return "";
  const titleMap: Record<DeviceAction, string> = {
    overview: "设备详情",
    topics: "主题",
    messages: "消息",
    publish: "发布",
    schedule: "定时",
    capabilities: "能力",
    type: "类型"
  };
  return `${titleMap[state.activeAction]} · ${state.selectedDevice.clientId}`;
});

onMounted(async () => {
  await restoreSession();
});

function emptyPage<T>(): Page<T> {
  return { items: [], page: 1, pageSize: 50, total: 0 };
}

async function restoreSession() {
  session.loading = true;
  try {
    session.user = await api.me();
    await refreshAll();
  } catch {
    session.user = null;
  } finally {
    session.loading = false;
  }
}

async function login() {
  session.loginError = "";
  try {
    const res = await api.login(session.username, session.password);
    session.user = res.user;
    session.password = "";
    await refreshAll();
  } catch (err) {
    session.loginError = errorMessage(err);
  }
}

async function logout() {
  await api.logout();
  session.user = null;
  closeSheet();
}

async function refreshAll() {
  state.error = "";
  state.loading = true;
  try {
    await Promise.all([loadDevices(), loadAlerts(), loadDeviceTypes()]);
  } catch (err) {
    state.error = errorMessage(err);
  } finally {
    state.loading = false;
  }
}

async function loadDevices() {
  const params = new URLSearchParams();
  params.set("pageSize", "100");
  if (state.q) params.set("q", state.q);
  if (state.status) params.set("status", state.status);
  state.devices = await api.devices(`?${params.toString()}`);
  if (state.selectedDevice) {
    const updated = state.devices.items.find((item) => item.id === state.selectedDevice?.id);
    if (updated) state.selectedDevice = updated;
  }
}

async function loadAlerts() {
  state.alerts = await api.alerts();
}

async function loadDeviceTypes() {
  state.deviceTypes = await api.deviceTypes();
}

async function loadDeviceTopics(device: Device) {
  state.deviceTopics = await api.deviceTopics(device.id);
}

async function loadDeviceMessages(device: Device) {
  const params = new URLSearchParams();
  params.set("pageSize", "80");
  params.set("deviceId", device.id);
  state.messages = await api.messages(`?${params.toString()}`);
}

async function openAction(device: Device, action: DeviceAction) {
  state.error = "";
  state.selectedDevice = device;
  state.activeAction = action;
  publishForm.result = "";
  publishForm.error = "";
  capabilityForm.result = "";
  capabilityForm.error = "";

  if (action === "topics" || action === "publish" || action === "schedule" || action === "capabilities" || action === "overview") {
    await loadDeviceTopics(device);
  }
  if (action === "messages" || action === "overview") {
    await loadDeviceMessages(device);
  }
  if (action === "schedule") {
    await loadScheduledTasks(device);
    seedScheduleForm();
  }
  if (action === "capabilities") {
    seedCapabilityForm();
  }
  if (action === "publish" && !publishForm.topic) {
    const firstPublishable = state.deviceTopics.items.find((item) => canPublishTopic(item.topic));
    publishForm.topic = firstPublishable?.topic ?? "";
  }
}

async function loadScheduledTasks(device: Device) {
  state.scheduledTasks = await api.scheduledPublishes(device.id);
}

function closeSheet() {
  state.activeAction = null;
  state.selectedDevice = null;
  state.showAlerts = false;
}

async function changeDeviceType(event: Event) {
  if (!state.selectedDevice) return;
  const target = event.target as HTMLSelectElement;
  const updated = await api.changeDeviceType(state.selectedDevice.id, target.value);
  state.selectedDevice = updated;
  await loadDevices();
}

async function publishMessage() {
  publishForm.error = "";
  publishForm.result = "";
  try {
    const res = await api.publish(publishForm.topic, publishForm.payload, publishForm.qos, publishForm.retain);
    publishForm.result = `已发布 ${res.commandId}`;
    if (state.selectedDevice) {
      await Promise.all([loadDeviceTopics(state.selectedDevice), loadDeviceMessages(state.selectedDevice)]);
    }
  } catch (err) {
    publishForm.error = errorMessage(err);
  }
}

function setPublishTopic(topic: string) {
  publishForm.topic = topic;
  state.activeAction = "publish";
}

function seedCapabilityForm() {
  const firstPublishable = state.deviceTopics.items.find((item) => canPublishTopic(item.topic));
  capabilityForm.commandId = "emit";
  capabilityForm.topic = firstPublishable?.topic ?? "";
  capabilityForm.messageId = generateMessageId();
  capabilityForm.no = 110;
  capabilityForm.result = "";
  capabilityForm.error = "";
}

function seedScheduleForm() {
  const firstPublishable = state.deviceTopics.items.find((item) => canPublishTopic(item.topic));
  scheduleForm.name = "";
  scheduleForm.topic = firstPublishable?.topic ?? "";
  scheduleForm.payload = "";
  scheduleForm.qos = 0;
  scheduleForm.retain = false;
  scheduleForm.scheduleType = "once";
  scheduleForm.timeOfDay = "09:00";
  scheduleForm.weekdays = [1, 2, 3, 4, 5];
  scheduleForm.result = "";
  scheduleForm.error = "";
  const next = new Date(Date.now() + 10 * 60 * 1000);
  scheduleForm.runAtLocal = toLocalDateTimeValue(next);
}

async function createScheduledPublish() {
  if (!state.selectedDevice) return;
  scheduleForm.error = "";
  scheduleForm.result = "";
  try {
    const runAt =
      scheduleForm.scheduleType === "once" && scheduleForm.runAtLocal
        ? new Date(scheduleForm.runAtLocal).toISOString()
        : undefined;
    const task = await api.createScheduledPublish({
      deviceId: state.selectedDevice.id,
      name: scheduleForm.name,
      topic: scheduleForm.topic,
      payload: scheduleForm.payload,
      qos: scheduleForm.qos,
      retain: scheduleForm.retain,
      scheduleType: scheduleForm.scheduleType,
      runAt,
      timeOfDay: scheduleForm.scheduleType === "once" ? undefined : scheduleForm.timeOfDay,
      weekdays: scheduleForm.scheduleType === "weekly" ? scheduleForm.weekdays : undefined,
      timezone: "Asia/Hong_Kong",
      payloadEncoding: "utf8"
    });
    scheduleForm.result = `已创建 ${task.id}`;
    await loadScheduledTasks(state.selectedDevice);
  } catch (err) {
    scheduleForm.error = errorMessage(err);
  }
}

async function cancelScheduledPublish(taskId: string) {
  if (!state.selectedDevice) return;
  await api.cancelScheduledPublish(taskId);
  await loadScheduledTasks(state.selectedDevice);
}

function selectCapabilityCommand(commandId: CapabilityCommandId) {
  capabilityForm.commandId = commandId;
  capabilityForm.result = "";
  capabilityForm.error = "";
}

function buildCapabilityPayload(): Record<string, unknown> {
  const no = Number(capabilityForm.no);
  let payload: Record<string, unknown>;
  switch (capabilityForm.commandId) {
    case "emit":
      payload = { action: "emit", data: { no }, type: "infrared" };
      break;
    case "erase":
      payload = { action: "erase", type: "infrared" };
      break;
    case "learn":
      payload = { action: "learn", data: { no }, type: "infrared" };
      break;
    case "learnBatch":
      payload = { action: "learnBatch", data: { file: capabilityForm.fileUrl.trim() }, type: "infrared" };
      break;
    case "learnCancel":
      payload = { action: "learnCancel", type: "infrared" };
      break;
    case "reset":
      payload = { system: "reset", type: "setting" };
      break;
    case "restart":
      payload = { system: "restart", type: "setting" };
      break;
    case "info":
      payload = { type: "info" };
      break;
    case "read":
      payload = { action: "ir_read", data: { no }, type: "infrared" };
      break;
    case "write":
      payload = { action: "ir_write", ir_data: capabilityForm.irData.trim(), no, type: "infrared" };
      break;
    case "mqtt":
      payload = {
        clientId: capabilityForm.mqttClientId.trim(),
        password: capabilityForm.mqttPassword,
        port: capabilityForm.mqttPort.trim(),
        protocol: "mqtt",
        publish: capabilityForm.mqttPublish.trim(),
        server: capabilityForm.mqttServer.trim(),
        subcribe: capabilityForm.mqttSubscribe.trim(),
        type: "custom",
        username: capabilityForm.mqttUsername.trim()
      };
      break;
    case "tcp":
      payload = {
        port: capabilityForm.tcpPort.trim(),
        protocol: "tcp",
        server: capabilityForm.tcpServer.trim(),
        type: "custom"
      };
      break;
    case "wifiLock":
      payload = { type: "setting", wifiLock: Number(capabilityForm.wifiLock) };
      break;
  }
  const messageId = capabilityForm.messageId.trim();
  return messageId ? { ...payload, messageId } : payload;
}

function validateCapabilityForm() {
  const command = selectedCapability.value;
  if (!capabilityForm.topic.trim()) return "请填写发布主题";
  if (command.needsNo && (!Number.isInteger(Number(capabilityForm.no)) || capabilityForm.no < 1 || capabilityForm.no > 248)) {
    return "红外码编号范围为 1~248";
  }
  if (command.needsFile && !capabilityForm.fileUrl.trim()) return "请填写红外码文件地址";
  if (command.needsIrData && !capabilityForm.irData.trim()) return "请填写红外码 HEX 数据";
  if (command.connection === "mqtt") {
    if (!capabilityForm.mqttServer.trim() || !capabilityForm.mqttPort.trim()) return "请填写 MQTT 服务器和端口";
    if (!capabilityForm.mqttClientId.trim()) return "请填写 MQTT ClientID";
    if (!capabilityForm.mqttPublish.trim() || !capabilityForm.mqttSubscribe.trim()) return "请填写 MQTT 发布和订阅主题";
  }
  if (command.connection === "tcp" && (!capabilityForm.tcpServer.trim() || !capabilityForm.tcpPort.trim())) {
    return "请填写 TCP 服务器和端口";
  }
  return "";
}

async function publishCapabilityCommand() {
  if (!state.selectedDevice) return;
  capabilityForm.error = "";
  capabilityForm.result = "";
  const validationError = validateCapabilityForm();
  if (validationError) {
    capabilityForm.error = validationError;
    return;
  }
  if (capabilityForm.commandId === "reset" && !window.confirm("恢复出厂会清空配网和自定义信息，确认继续？")) {
    return;
  }
  try {
    const payload = JSON.stringify(buildCapabilityPayload(), null, 2);
    const res = await api.publish(capabilityForm.topic.trim(), payload, capabilityForm.qos, capabilityForm.retain);
    capabilityForm.result = `已发布 ${res.commandId}`;
    await Promise.all([loadDeviceTopics(state.selectedDevice), loadDeviceMessages(state.selectedDevice)]);
  } catch (err) {
    capabilityForm.error = errorMessage(err);
  }
}

function toggleWeekday(day: number) {
  if (scheduleForm.weekdays.includes(day)) {
    scheduleForm.weekdays = scheduleForm.weekdays.filter((item) => item !== day);
  } else {
    scheduleForm.weekdays = [...scheduleForm.weekdays, day].sort((a, b) => a - b);
  }
}

function toLocalDateTimeValue(date: Date) {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function generateMessageId() {
  const pad = (value: number, length = 2) => String(value).padStart(length, "0");
  const now = new Date();
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}${pad(now.getMilliseconds(), 3)}`;
}

function isInfraredController(device: Device) {
  return device.type === GSCU1B_TYPE;
}

function canPublishTopic(topic: string) {
  return topic !== "" && !topic.includes("#") && !topic.includes("+") && !topic.toUpperCase().startsWith("$SYS");
}

function errorMessage(err: unknown) {
  if (err instanceof ApiError) return err.message;
  if (err instanceof Error) return err.message;
  return "请求失败";
}

function statusLabel(status: Device["status"]) {
  if (status === "online") return "在线";
  if (status === "stale_offline") return "过期离线";
  return "离线";
}

function formatDate(value?: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  }).format(new Date(value));
}
</script>

<template>
  <div v-if="session.loading" class="boot">
    <RefreshCw class="spin" :size="20" />
  </div>

  <main v-else-if="!session.user" class="login-shell">
    <section class="login-panel">
      <div class="brand-row">
        <div class="brand-mark"><TerminalSquare :size="24" /></div>
        <div>
          <h1>mqtter</h1>
          <p>MQTT 管理后台</p>
        </div>
      </div>
      <form class="login-form" @submit.prevent="login">
        <label>
          <span>账号</span>
          <input v-model="session.username" autocomplete="username" />
        </label>
        <label>
          <span>密码</span>
          <input v-model="session.password" type="password" autocomplete="current-password" autofocus />
        </label>
        <button class="primary" type="submit">
          <Check :size="17" />
          登录
        </button>
        <p v-if="session.loginError" class="error-line">{{ session.loginError }}</p>
      </form>
    </section>
  </main>

  <main v-else class="app-shell">
    <header class="topbar">
      <div class="product">
        <div class="brand-mark small"><TerminalSquare :size="20" /></div>
        <div>
          <h1>mqtter</h1>
          <p>{{ session.user.username }} · {{ session.user.role }}</p>
        </div>
      </div>
      <div class="top-actions">
        <button class="tool-button" title="告警" @click="state.showAlerts = true">
          <Bell :size="18" />
          <span>{{ alertCount }}</span>
        </button>
        <button class="icon-button" title="刷新" @click="refreshAll">
          <RefreshCw :class="{ spin: state.loading }" :size="18" />
        </button>
        <button class="icon-button" title="退出" @click="logout">
          <LogOut :size="18" />
        </button>
      </div>
    </header>

    <section class="metrics">
      <div class="metric">
        <Wifi :size="18" />
        <span>在线</span>
        <strong>{{ onlineCount }}</strong>
      </div>
      <div class="metric">
        <WifiOff :size="18" />
        <span>离线</span>
        <strong>{{ offlineCount }}</strong>
      </div>
      <div class="metric">
        <Router :size="18" />
        <span>设备</span>
        <strong>{{ state.devices.total }}</strong>
      </div>
      <div class="metric warn">
        <AlertTriangle :size="18" />
        <span>告警</span>
        <strong>{{ alertCount }}</strong>
      </div>
    </section>

    <section class="device-board">
      <div class="board-head">
        <div>
          <h2>设备列表</h2>
          <p>ClientID 作为设备身份，默认类型为 unknown</p>
        </div>
        <div class="filters">
          <div class="searchbox">
            <Search :size="17" />
            <input v-model="state.q" placeholder="ClientID" @keyup.enter="loadDevices" />
          </div>
          <select v-model="state.status" @change="loadDevices">
            <option value="">全部状态</option>
            <option value="online">在线</option>
            <option value="offline">离线</option>
            <option value="stale_offline">过期离线</option>
          </select>
        </div>
      </div>

      <p v-if="state.error" class="error-line">{{ state.error }}</p>

      <div class="device-table">
        <div class="device-header">
          <span>设备</span>
          <span>状态</span>
          <span>类型</span>
          <span>最后活跃</span>
          <span>操作</span>
        </div>

        <article v-for="device in state.devices.items" :key="device.id" class="device-row">
          <div class="device-main">
            <span class="status-dot" :class="device.status"></span>
            <div>
              <strong class="mono">{{ device.clientId }}</strong>
              <p class="mono">{{ device.sessionId || "-" }}</p>
            </div>
          </div>
          <span class="pill" :class="device.status">{{ statusLabel(device.status) }}</span>
          <span class="type-chip">{{ device.type }}</span>
          <span>{{ formatDate(device.lastSeenAt) }}</span>
          <div class="row-actions">
            <button title="详情" @click="openAction(device, 'overview')">
              <Info :size="16" />
              详情
            </button>
            <button title="主题" @click="openAction(device, 'topics')">
              <Layers3 :size="16" />
              主题
            </button>
            <button title="消息" @click="openAction(device, 'messages')">
              <MessageSquare :size="16" />
              消息
            </button>
            <button title="发布" @click="openAction(device, 'publish')">
              <Send :size="16" />
              发布
            </button>
            <button title="定时" @click="openAction(device, 'schedule')">
              <Clock3 :size="16" />
              定时
            </button>
            <button v-if="isInfraredController(device)" title="设备能力" @click="openAction(device, 'capabilities')">
              <RadioTower :size="16" />
              能力
            </button>
            <button title="类型" @click="openAction(device, 'type')">
              <Settings2 :size="16" />
              类型
            </button>
          </div>
        </article>

        <div v-if="state.devices.items.length === 0" class="empty">暂无设备</div>
      </div>
    </section>

    <div v-if="state.activeAction && state.selectedDevice" class="sheet-backdrop" @click.self="closeSheet">
      <section class="sheet">
        <header class="sheet-head">
          <div>
            <h2>{{ actionTitle }}</h2>
            <p>{{ statusLabel(state.selectedDevice.status) }} · {{ formatDate(state.selectedDevice.lastSeenAt) }}</p>
          </div>
          <button class="icon-button" title="关闭" @click="closeSheet">
            <X :size="18" />
          </button>
        </header>

        <div v-if="state.activeAction === 'overview'" class="sheet-body overview">
          <div class="info-grid">
            <div>
              <span>ClientID</span>
              <strong class="mono">{{ state.selectedDevice.clientId }}</strong>
            </div>
            <div>
              <span>Session</span>
              <strong class="mono">{{ state.selectedDevice.sessionId || "-" }}</strong>
            </div>
            <div>
              <span>类型</span>
              <strong>{{ state.selectedDevice.type }}</strong>
            </div>
            <div>
              <span>首次接入</span>
              <strong>{{ formatDate(state.selectedDevice.firstSeenAt) }}</strong>
            </div>
          </div>
          <div class="split-list">
            <section>
              <h3><Layers3 :size="16" /> 主题</h3>
              <button v-for="topic in state.deviceTopics.items.slice(0, 8)" :key="`${topic.kind}:${topic.topic}`" class="topic-line" :disabled="!canPublishTopic(topic.topic)" @click="setPublishTopic(topic.topic)">
                <span class="kind">{{ topic.kind }}</span>
                <span class="mono">{{ topic.topic }}</span>
              </button>
              <div v-if="state.deviceTopics.items.length === 0" class="empty compact">暂无主题</div>
            </section>
            <section>
              <h3><MessageSquare :size="16" /> 最近消息</h3>
              <article v-for="message in state.messages.items.slice(0, 5)" :key="message.id" class="message-card">
                <span class="mono">{{ message.topic }}</span>
                <pre>{{ message.payloadText }}</pre>
              </article>
              <div v-if="state.messages.items.length === 0" class="empty compact">暂无消息</div>
            </section>
          </div>
        </div>

        <div v-if="state.activeAction === 'topics'" class="sheet-body">
          <div class="list-stack">
            <article v-for="topic in state.deviceTopics.items" :key="`${topic.kind}:${topic.topic}`" class="topic-item">
              <div>
                <span class="kind">{{ topic.kind }}</span>
                <strong class="mono">{{ topic.topic }}</strong>
              </div>
              <div class="topic-meta">
                <span>QoS {{ topic.qos }}</span>
                <span>{{ formatDate(topic.lastSeenAt) }}</span>
                <button :disabled="!canPublishTopic(topic.topic)" @click="setPublishTopic(topic.topic)">
                  <Send :size="15" />
                  发布
                </button>
              </div>
            </article>
            <div v-if="state.deviceTopics.items.length === 0" class="empty">暂无主题</div>
          </div>
        </div>

        <div v-if="state.activeAction === 'messages'" class="sheet-body">
          <div class="message-list">
            <article v-for="message in state.messages.items" :key="message.id" class="message-row">
              <header>
                <span class="mono">{{ message.topic }}</span>
                <span>{{ formatDate(message.receivedAt) }}</span>
              </header>
              <pre>{{ message.payloadText }}</pre>
              <footer>
                <span>{{ message.payloadFormat }}</span>
                <span>QoS {{ message.qos }}</span>
                <span v-if="message.retain">retain</span>
              </footer>
            </article>
            <div v-if="state.messages.items.length === 0" class="empty">暂无消息</div>
          </div>
        </div>

        <form v-if="state.activeAction === 'publish'" class="sheet-body publish-form" @submit.prevent="publishMessage">
          <label class="field">
            <span>主题</span>
            <input v-model="publishForm.topic" placeholder="devices/demo/in" />
          </label>
          <label class="field">
            <span>Payload</span>
            <textarea v-model="publishForm.payload" spellcheck="false" placeholder='{"cmd":"ping"}'></textarea>
          </label>
          <div class="form-row">
            <label class="field compact">
              <span>QoS</span>
              <select v-model.number="publishForm.qos">
                <option :value="0">0</option>
                <option :value="1">1</option>
              </select>
            </label>
            <label class="toggle">
              <input v-model="publishForm.retain" type="checkbox" />
              <span>retain</span>
            </label>
          </div>
          <button class="primary" type="submit">
            <Send :size="17" />
            发布消息
          </button>
          <p v-if="publishForm.result" class="success-line">{{ publishForm.result }}</p>
          <p v-if="publishForm.error" class="error-line">{{ publishForm.error }}</p>
        </form>

        <div v-if="state.activeAction === 'schedule'" class="sheet-body schedule-layout">
          <form class="schedule-form" @submit.prevent="createScheduledPublish">
            <label class="field">
              <span>任务名称</span>
              <input v-model="scheduleForm.name" placeholder="巡检命令" />
            </label>
            <label class="field">
              <span>主题</span>
              <input v-model="scheduleForm.topic" placeholder="devices/demo/in" />
            </label>
            <label class="field">
              <span>Payload</span>
              <textarea v-model="scheduleForm.payload" spellcheck="false" placeholder='{"cmd":"ping"}'></textarea>
            </label>
            <div class="form-row">
              <label class="field compact">
                <span>QoS</span>
                <select v-model.number="scheduleForm.qos">
                  <option :value="0">0</option>
                  <option :value="1">1</option>
                </select>
              </label>
              <label class="toggle">
                <input v-model="scheduleForm.retain" type="checkbox" />
                <span>retain</span>
              </label>
            </div>
            <div class="schedule-mode">
              <button type="button" :class="{ active: scheduleForm.scheduleType === 'once' }" @click="scheduleForm.scheduleType = 'once'">一次性</button>
              <button type="button" :class="{ active: scheduleForm.scheduleType === 'daily' }" @click="scheduleForm.scheduleType = 'daily'">每天</button>
              <button type="button" :class="{ active: scheduleForm.scheduleType === 'weekly' }" @click="scheduleForm.scheduleType = 'weekly'">按周</button>
            </div>
            <label v-if="scheduleForm.scheduleType === 'once'" class="field">
              <span>执行时间</span>
              <input v-model="scheduleForm.runAtLocal" type="datetime-local" />
            </label>
            <label v-if="scheduleForm.scheduleType !== 'once'" class="field compact-time">
              <span>执行时刻</span>
              <input v-model="scheduleForm.timeOfDay" type="time" />
            </label>
            <div v-if="scheduleForm.scheduleType === 'weekly'" class="weekday-picker">
              <button type="button" :class="{ active: scheduleForm.weekdays.includes(1) }" @click="toggleWeekday(1)">周一</button>
              <button type="button" :class="{ active: scheduleForm.weekdays.includes(2) }" @click="toggleWeekday(2)">周二</button>
              <button type="button" :class="{ active: scheduleForm.weekdays.includes(3) }" @click="toggleWeekday(3)">周三</button>
              <button type="button" :class="{ active: scheduleForm.weekdays.includes(4) }" @click="toggleWeekday(4)">周四</button>
              <button type="button" :class="{ active: scheduleForm.weekdays.includes(5) }" @click="toggleWeekday(5)">周五</button>
              <button type="button" :class="{ active: scheduleForm.weekdays.includes(6) }" @click="toggleWeekday(6)">周六</button>
              <button type="button" :class="{ active: scheduleForm.weekdays.includes(7) }" @click="toggleWeekday(7)">周日</button>
            </div>
            <button class="primary" type="submit">
              <Clock3 :size="17" />
              创建定时任务
            </button>
            <p v-if="scheduleForm.result" class="success-line">{{ scheduleForm.result }}</p>
            <p v-if="scheduleForm.error" class="error-line">{{ scheduleForm.error }}</p>
          </form>

          <section class="scheduled-list">
            <h3><Clock3 :size="16" /> 已有任务</h3>
            <article v-for="task in state.scheduledTasks.items" :key="task.id" class="scheduled-card" :class="task.status">
              <div>
                <strong>{{ task.name || task.topic }}</strong>
                <p class="mono">{{ task.topic }}</p>
              </div>
              <div class="scheduled-meta">
                <span>{{ task.scheduleType }}</span>
                <span>{{ task.status }}</span>
                <span>下次 {{ formatDate(task.nextRunAt) }}</span>
                <span>已执行 {{ task.runCount }}</span>
              </div>
              <p v-if="task.lastError" class="error-line">{{ task.lastError }}</p>
              <button v-if="task.status === 'active'" type="button" @click="cancelScheduledPublish(task.id)">
                <X :size="15" />
                取消
              </button>
            </article>
            <div v-if="state.scheduledTasks.items.length === 0" class="empty compact">暂无定时任务</div>
          </section>
        </div>

        <div v-if="state.activeAction === 'capabilities'" class="sheet-body capability-layout">
          <section class="capability-command-list">
            <button
              v-for="item in infraredCapabilities"
              :key="item.id"
              type="button"
              :class="{ active: capabilityForm.commandId === item.id }"
              @click="selectCapabilityCommand(item.id)"
            >
              <strong>{{ item.title }}</strong>
              <span>{{ item.commandName }}</span>
            </button>
          </section>

          <form class="capability-form" @submit.prevent="publishCapabilityCommand">
            <div>
              <h3>{{ selectedCapability.title }}</h3>
              <p>{{ selectedCapability.description }}</p>
            </div>
            <label class="field">
              <span>发布主题</span>
              <input v-model="capabilityForm.topic" placeholder="devices/demo/in" />
            </label>
            <div class="form-row">
              <label class="field compact">
                <span>QoS</span>
                <select v-model.number="capabilityForm.qos">
                  <option :value="0">0</option>
                  <option :value="1">1</option>
                </select>
              </label>
              <label class="toggle">
                <input v-model="capabilityForm.retain" type="checkbox" />
                <span>retain</span>
              </label>
            </div>
            <label class="field">
              <span>消息ID</span>
              <input v-model="capabilityForm.messageId" placeholder="可选，用于设备回包匹配" />
            </label>
            <label v-if="selectedCapability.needsNo" class="field compact">
              <span>红外码编号</span>
              <input v-model.number="capabilityForm.no" type="number" min="1" max="248" />
            </label>
            <label v-if="selectedCapability.needsFile" class="field">
              <span>红外码文件地址</span>
              <input v-model="capabilityForm.fileUrl" placeholder="http://geek-smart-pub.oss-cn-qingdao.aliyuncs.com/new.bin" />
            </label>
            <label v-if="selectedCapability.needsIrData" class="field">
              <span>红外码 HEX 数据</span>
              <textarea v-model="capabilityForm.irData" class="payload-preview" spellcheck="false" placeholder="00495D...FFFFFFFF"></textarea>
            </label>
            <template v-if="selectedCapability.connection === 'mqtt'">
              <div class="form-row">
                <label class="field">
                  <span>MQTT 服务器</span>
                  <input v-model="capabilityForm.mqttServer" />
                </label>
                <label class="field compact">
                  <span>端口</span>
                  <input v-model="capabilityForm.mqttPort" />
                </label>
              </div>
              <div class="form-row">
                <label class="field">
                  <span>ClientID</span>
                  <input v-model="capabilityForm.mqttClientId" />
                </label>
                <label class="field">
                  <span>用户名</span>
                  <input v-model="capabilityForm.mqttUsername" />
                </label>
              </div>
              <label class="field">
                <span>密码</span>
                <input v-model="capabilityForm.mqttPassword" type="password" />
              </label>
              <div class="form-row">
                <label class="field">
                  <span>发布主题</span>
                  <input v-model="capabilityForm.mqttPublish" />
                </label>
                <label class="field">
                  <span>订阅主题</span>
                  <input v-model="capabilityForm.mqttSubscribe" />
                </label>
              </div>
            </template>
            <template v-if="selectedCapability.connection === 'tcp'">
              <div class="form-row">
                <label class="field">
                  <span>TCP 服务器</span>
                  <input v-model="capabilityForm.tcpServer" />
                </label>
                <label class="field compact">
                  <span>端口</span>
                  <input v-model="capabilityForm.tcpPort" />
                </label>
              </div>
            </template>
            <label v-if="selectedCapability.setting === 'wifiLock'" class="field compact">
              <span>配网锁</span>
              <select v-model.number="capabilityForm.wifiLock">
                <option :value="0">关闭</option>
                <option :value="1">开启</option>
              </select>
            </label>
            <p v-if="selectedCapability.danger" class="error-line">恢复出厂会清空配网信息和自定义信息，请确认现场允许执行。</p>
            <label class="field">
              <span>Payload 预览</span>
              <textarea class="payload-preview" :value="capabilityPayloadPreview" readonly spellcheck="false"></textarea>
            </label>
            <button class="primary" type="submit">
              <Send :size="17" />
              发布能力指令
            </button>
            <p v-if="capabilityForm.result" class="success-line">{{ capabilityForm.result }}</p>
            <p v-if="capabilityForm.error" class="error-line">{{ capabilityForm.error }}</p>
          </form>
        </div>

        <div v-if="state.activeAction === 'type'" class="sheet-body type-editor">
          <FileText :size="28" />
          <label class="field">
            <span>设备类型</span>
            <select :value="state.selectedDevice.type" @change="changeDeviceType">
              <option v-for="item in state.deviceTypes" :key="item.code" :value="item.code">
                {{ item.name }}
              </option>
            </select>
          </label>
        </div>
      </section>
    </div>

    <div v-if="state.showAlerts" class="sheet-backdrop" @click.self="closeSheet">
      <section class="sheet narrow">
        <header class="sheet-head">
          <div>
            <h2>系统告警</h2>
            <p>{{ alertCount }} open</p>
          </div>
          <button class="icon-button" title="关闭" @click="closeSheet">
            <X :size="18" />
          </button>
        </header>
        <div class="sheet-body">
          <article v-for="alert in state.alerts.items" :key="alert.id" class="alert-row" :class="alert.level">
            <Gauge :size="17" />
            <div>
              <strong>{{ alert.code }}</strong>
              <p>{{ alert.message }}</p>
            </div>
            <span>{{ formatDate(alert.lastSeenAt) }}</span>
          </article>
          <div v-if="state.alerts.items.length === 0" class="empty">暂无告警</div>
        </div>
      </section>
    </div>
  </main>
</template>
