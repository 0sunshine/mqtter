import type {
  CreateQuickAction,
  CreateScheduledPublish,
  Device,
  DeviceType,
  Message,
  ObservedTopic,
  Page,
  QuickAction,
  QuickActionExecuteResult,
  PublishResult,
  ScheduledPublishTask,
  SystemAlert,
  User
} from "./types";

export class ApiError extends Error {
  code: string;

  constructor(code: string, message: string) {
    super(message);
    this.code = code;
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init.headers ?? {})
    }
  });
  if (!response.ok) {
    let code = "request_failed";
    let message = `HTTP ${response.status}`;
    try {
      const body = await response.json();
      code = body?.error?.code ?? code;
      message = body?.error?.message ?? message;
    } catch {
      // ignore non-json error responses
    }
    throw new ApiError(code, message);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

export const api = {
  login(username: string, password: string) {
    return request<{ user: User }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password })
    });
  },
  logout() {
    return request<void>("/api/auth/logout", { method: "POST" });
  },
  me() {
    return request<User>("/api/me");
  },
  devices(params = "") {
    return request<Page<Device>>(`/api/devices${params}`);
  },
  deviceTypes() {
    return request<DeviceType[]>("/api/device-types");
  },
  changeDeviceType(deviceId: string, type: string) {
    return request<Device>(`/api/devices/${deviceId}/type`, {
      method: "PATCH",
      body: JSON.stringify({ type })
    });
  },
  deviceTopics(deviceId: string) {
    return request<Page<ObservedTopic>>(`/api/devices/${deviceId}/topics?pageSize=100`);
  },
  topics(params = "") {
    return request<Page<ObservedTopic>>(`/api/topics${params}`);
  },
  messages(params = "") {
    return request<Page<Message>>(`/api/messages${params}`);
  },
  publish(topic: string, payload: string, qos: number, retain: boolean) {
    return request<PublishResult>("/api/publish", {
      method: "POST",
      body: JSON.stringify({ topic, payload, qos, retain, payloadEncoding: "utf8" })
    });
  },
  scheduledPublishes(deviceId: string) {
    const params = new URLSearchParams({ deviceId, pageSize: "50" });
    return request<Page<ScheduledPublishTask>>(`/api/scheduled-publishes?${params.toString()}`);
  },
  createScheduledPublish(payload: CreateScheduledPublish) {
    return request<ScheduledPublishTask>("/api/scheduled-publishes", {
      method: "POST",
      body: JSON.stringify(payload)
    });
  },
  cancelScheduledPublish(taskId: string) {
    return request<ScheduledPublishTask>(`/api/scheduled-publishes/${taskId}/cancel`, {
      method: "POST"
    });
  },
  quickActions(deviceId: string) {
    const params = new URLSearchParams({ deviceId, pageSize: "50" });
    return request<Page<QuickAction>>(`/api/quick-actions?${params.toString()}`);
  },
  createQuickAction(payload: CreateQuickAction) {
    return request<QuickAction>("/api/quick-actions", {
      method: "POST",
      body: JSON.stringify(payload)
    });
  },
  deleteQuickAction(actionId: string) {
    return request<void>(`/api/quick-actions/${actionId}`, {
      method: "DELETE"
    });
  },
  executeQuickAction(actionId: string) {
    return request<QuickActionExecuteResult>(`/api/quick-actions/${actionId}/execute`, {
      method: "POST"
    });
  },
  alerts() {
    return request<Page<SystemAlert>>("/api/alerts?pageSize=20");
  }
};
