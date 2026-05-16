export type Page<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
};

export type User = {
  id: string;
  username: string;
  role: string;
};

export type Device = {
  id: string;
  clientId: string;
  type: string;
  status: "online" | "offline" | "stale_offline";
  sessionId?: string;
  firstSeenAt: string;
  lastSeenAt: string;
  lastDisconnectedAt?: string;
};

export type ObservedTopic = {
  deviceId?: string;
  clientId: string;
  topic: string;
  kind: "subscribe_filter" | "publish_topic" | "admin_publish_topic";
  qos: number;
  firstSeenAt: string;
  lastSeenAt: string;
};

export type Message = {
  id: string;
  deviceId?: string;
  clientId: string;
  sessionId?: string;
  topic: string;
  payloadText: string;
  payloadFormat: "text" | "json";
  qos: number;
  retain: boolean;
  receivedAt: string;
};

export type DeviceType = {
  code: string;
  name: string;
  description: string;
};

export type SystemAlert = {
  id: string;
  level: "info" | "warning" | "critical";
  code: string;
  message: string;
  status: "open" | "resolved";
  firstSeenAt: string;
  lastSeenAt: string;
};

export type PublishResult = {
  commandId: string;
  status: string;
  publishedAt: string;
};

export type ScheduleType = "once" | "daily" | "weekly";

export type ScheduledPublishTask = {
  id: string;
  deviceId: string;
  clientId?: string;
  adminUserId: string;
  name: string;
  topic: string;
  payload: string;
  qos: number;
  retain: boolean;
  scheduleType: ScheduleType;
  runAt?: string;
  timeOfDay?: string;
  weekdays?: number[];
  timezone: string;
  status: "active" | "canceled" | "completed" | "failed";
  nextRunAt?: string;
  lastRunAt?: string;
  lastError?: string;
  runCount: number;
  createdAt: string;
  updatedAt: string;
};

export type CreateScheduledPublish = {
  deviceId: string;
  name: string;
  topic: string;
  payload: string;
  qos: number;
  retain: boolean;
  scheduleType: ScheduleType;
  runAt?: string;
  timeOfDay?: string;
  weekdays?: number[];
  timezone: string;
  payloadEncoding: "utf8";
};
