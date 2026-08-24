import React from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Typography,
} from 'antd';
import { ArrowLeftOutlined, SaveOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { apiGet, apiPost, apiPut } from '../api/client';
import type { Connection, ConnectionInput, ConnectionTestResult, ConnType } from '../types';

const sslOptions = [
  { value: 'prefer', label: 'Prefer' },
  { value: 'require', label: 'Require' },
  { value: 'verify-ca', label: 'Verify CA' },
  { value: 'verify-full', label: 'Verify Full' },
  { value: 'disable', label: 'Disable' },
];

export default function ConnectionFormPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { message } = App.useApp();
  const [form] = Form.useForm<ConnectionInput>();
  const [saving, setSaving] = React.useState(false);
  const [testing, setTesting] = React.useState(false);
  const [initialized, setInitialized] = React.useState(false);
  const editing = Boolean(id);

  React.useEffect(() => {
    if (!id) {
      setInitialized(true);
      return;
    }
    apiGet<Connection>(`/connections/${id}`)
      .then((c) => {
        form.setFieldsValue({
          name: c.name,
          type: c.type,
          host: c.host,
          port: c.port,
          username: c.username,
          database: c.database,
          ssl_mode: c.ssl_mode || 'prefer',
          description: c.description,
        });
        setInitialized(true);
      })
      .catch((e: Error) => message.error(e.message));
  }, [id, form, message]);

  const test = async () => {
    let values: ConnectionInput;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setTesting(true);
    try {
      const r = await apiPost<ConnectionTestResult>('/connections/test', values);
      message.success(`Connection OK: ${r.version} (${r.latency_ms} ms)`);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setTesting(false);
    }
  };

  const save = async () => {
    let values: ConnectionInput;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setSaving(true);
    try {
      if (editing) {
        await apiPut<Connection>(`/connections/${id}`, values);
        message.success('Connection updated');
      } else {
        await apiPost<Connection>('/connections', values);
        message.success('Connection created');
      }
      navigate('/connections');
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/connections')}>
          Back
        </Button>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {editing ? 'Edit Connection' : 'New Connection'}
        </Typography.Title>
      </Space>
      <Card>
        {!initialized ? null : (
          <Form form={form} layout="vertical" initialValues={{ type: 'mysql', port: 3306, ssl_mode: 'prefer' }}>
            <Row gutter={16}>
              <Col xs={24} md={12}>
                <Form.Item name="name" label="Name" rules={[{ required: true, message: 'Name is required' }]}>
                  <Input placeholder="test-mysql" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item name="type" label="Database Type" rules={[{ required: true }]}>
                  <Select<ConnType>
                    options={[
                      { value: 'mysql', label: 'MySQL' },
                      { value: 'postgresql', label: 'PostgreSQL' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item name="host" label="Host" rules={[{ required: true, message: 'Host is required' }]}>
                  <Input placeholder="10.0.0.10" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item name="port" label="Port" rules={[{ required: true }]}>
                  <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item name="username" label="Username">
                  <Input placeholder="root" autoComplete="off" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item name="password" label="Password" rules={editing ? [] : [{ required: true, message: 'Password is required' }]}>
                  <Input.Password placeholder={editing ? 'Leave empty to keep the current password' : ''} autoComplete="new-password" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item name="database" label="Database">
                  <Input placeholder="order_db" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item name="ssl_mode" label="SSL Mode">
                  <Select options={sslOptions} />
                </Form.Item>
              </Col>
              <Col span={24}>
                <Form.Item name="description" label="Description">
                  <Input.TextArea rows={2} />
                </Form.Item>
              </Col>
            </Row>
            <Space>
              <Button icon={<ThunderboltOutlined />} loading={testing} onClick={test}>
                Test Connection
              </Button>
              <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={save}>
                Save
              </Button>
              <Button onClick={() => navigate('/connections')}>Cancel</Button>
            </Space>
          </Form>
        )}
      </Card>
    </div>
  );
}
