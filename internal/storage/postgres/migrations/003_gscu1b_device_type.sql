INSERT INTO device_types (code, name, description, schema_json)
VALUES (
  'GSCU1B',
  '智能红外控制器-WiFi版',
  'GSCU1B 智能红外控制器，支持红外学习、发射、读写码、设备信息、配网锁、重启、恢复出厂和自定义 MQTT/TCP。',
  '{"productCode":"GSCU1B","capabilities":["controller-infrared-emit","controller-infrared-erase","controller-infrared-learn","controller-infrared-learn-batch","controller-infrared-learn-cancel","controller-reset","controller-restart","info-all","ir_read","ir_write","setting-mqtt","setting-tcp","setting-wifi-lock"]}'
)
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    schema_json = EXCLUDED.schema_json;
