import React from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Divider,
  Form,
  Input,
  Radio,
  Row,
  Select,
  Space,
  Table,
  Typography,
} from 'antd';
import { ArrowLeftOutlined, ArrowRightOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { apiGet, apiPost } from '../api/client';
import type { Connection, DatabasePair, MigrationInput } from '../types';
import { ConnectionSummary } from '../components/ConnBadge';
import { buildMapping, validateMapping } from '../utils/mapping';

interface FormValues {
  name: string;
  source_connection_id: number;
  target_connection_id: number;
  migration_type: 'FULL';
  target_db_policy: MigrationInput['target_db_policy'];
}

export default function NewMigration() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [form] = Form.useForm<FormValues>();
  const [connections, setConnections] = React.useState<Connection[]>([]);
  const [sourceDBs, setSourceDBs] = React.useState<string[]>([]);
  const [mapping, setMapping] = React.useState<DatabasePair[]>([]);
  const [loadingDBs, setLoadingDBs] = React.useState(false);
  const [starting, setStarting] = React.useState(false);
  const sourceId = Form.useWatch('source_connection_id', form);
  const sourceType = connections.find((c) => c.id === sourceId)?.type;
  const targetId = Form.useWatch('target_connection_id', form);
  const targetType = connections.find((c) => c.id === targetId)?.type;

  React.useEffect(() => {
    apiGet<Connection[]>('/connections')
      .then(setConnections)
      .catch((e: Error) => message.error(e.message));
  }, [message]);

  const loadSourceDBs = async (id: number) => {
    setSourceDBs([]);
    setMapping([]);
    setLoadingDBs(true);
    try {
      const r = await apiGet<{ databases: string[] }>(`/connections/${id}/databases`);
      setSourceDBs(r.databases);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoadingDBs(false);
    }
  };

  const onSourceDBsChange = (dbs: string[]) => {
    setMapping(buildMapping(dbs, mapping));
  };

  const updateTarget = (index: number, target: string) => {
    setMapping((prev) => prev.map((p, i) => (i === index ? { ...p, target } : p)));
  };

  const start = async () => {
    let values: FormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    if (sourceType !== targetType) {
      message.error('MVP supports same-type migrations only (mysql->mysql, postgresql->postgresql)');
      return;
    }
    const pairs = mapping.filter((p) => p.source && p.target);
    const validationError = validateMapping(pairs);
    if (validationError) {
      message.error(validationError);
      return;
    }

    setStarting(true);
    try {
      const input: MigrationInput = {
        name: values.name,
        source_connection_id: values.source_connection_id,
        target_connection_id: values.target_connection_id,
        databases: pairs,
        migration_type: 'FULL',
        target_db_policy: values.target_db_policy,
        created_by: 'web',
      };
      const created = await apiPost<{ id: number; status: string }>('/migrations', input);
      await apiPost(`/migrations/${created.id}/start`);
      message.success(`Migration #${created.id} started (${pairs.length} database(s))`);
      navigate(`/migrations/${created.id}`);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setStarting(false);
    }
  };

  const mappingColumns = [
    {
      title: 'Source Database',
      dataIndex: 'source',
      render: (v: string) => <Typography.Text code>{v}</Typography.Text>,
    },
    {
      title: '→ Target Database',
      dataIndex: 'target',
      render: (v: string, _: unknown, index: number) => (
        <Input
          value={v}
          placeholder="target database name"
          onChange={(e) => updateTarget(index, e.target.value)}
        />
      ),
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/migrations')}>
          Back
        </Button>
        <Typography.Title level={4} style={{ margin: 0 }}>
          New Migration
        </Typography.Title>
      </Space>
      <Form
        form={form}
        layout="vertical"
        initialValues={{ migration_type: 'FULL', target_db_policy: 'error' }}
      >
        <Row gutter={16}>
          <Col xs={24} md={12}>
            <Card title="Source" styles={{ header: { color: '#1677ff' } }}>
              <Form.Item name="source_connection_id" label="Connection" rules={[{ required: true, message: 'Select source connection' }]}>
                <Select
                  showSearch
                  optionFilterProp="label"
                  placeholder="Select source connection"
                  options={connections.map((c) => ({
                    value: c.id,
                    label: `${c.name} (${c.type}://${c.host}:${c.port})`,
                  }))}
                  onChange={(v: number) => loadSourceDBs(v)}
                />
              </Form.Item>
              <Form.Item label="Databases" required>
                <Select
                  mode="multiple"
                  allowClear
                  showSearch
                  loading={loadingDBs}
                  placeholder="Select one or more source databases"
                  value={mapping.map((p) => p.source)}
                  onChange={onSourceDBsChange}
                  options={sourceDBs.map((d) => ({ value: d, label: d }))}
                />
              </Form.Item>
              {sourceId ? (
                <ConnectionSummary
                  type={sourceType || 'mysql'}
                  name={connections.find((c) => c.id === sourceId)?.name || ''}
                />
              ) : null}
            </Card>
          </Col>

          <Col xs={24} md={12}>
            <Card title="Target" styles={{ header: { color: '#52c41a' } }}>
              <Form.Item name="target_connection_id" label="Connection" rules={[{ required: true, message: 'Select target connection' }]}>
                <Select
                  showSearch
                  optionFilterProp="label"
                  placeholder="Select target connection"
                  options={connections.map((c) => ({
                    value: c.id,
                    label: `${c.name} (${c.type}://${c.host}:${c.port})`,
                  }))}
                />
              </Form.Item>
              <Form.Item name="target_db_policy" label="If target database exists">
                <Radio.Group>
                  <Radio value="error">Refuse (default)</Radio>
                  <Radio value="create">Create if missing</Radio>
                  <Radio value="overwrite">Overwrite</Radio>
                </Radio.Group>
              </Form.Item>
              {targetId ? (
                <ConnectionSummary
                  type={targetType || 'mysql'}
                  name={connections.find((c) => c.id === targetId)?.name || ''}
                />
              ) : null}
            </Card>
          </Col>
        </Row>

        <Card title="Source → Target Mapping" style={{ marginTop: 16 }}>
          <Table
            rowKey="source"
            size="small"
            columns={mappingColumns}
            dataSource={mapping}
            pagination={false}
            locale={{ emptyText: 'Select source databases to build the mapping' }}
          />
        </Card>

        <Card style={{ marginTop: 16 }}>
          <Form.Item name="name" label="Migration Name" rules={[{ required: true, message: 'Name is required' }]}>
            <Input placeholder="order-db-migration" />
          </Form.Item>
          <Form.Item name="migration_type" label="Migration Type">
            <Radio.Group>
              <Radio.Button value="FULL">Full Migration</Radio.Button>
            </Radio.Group>
          </Form.Item>
          <Divider plain>
            <ArrowRightOutlined /> Source → Target <ArrowRightOutlined />
          </Divider>
          <Space>
            <Button type="primary" size="large" icon={<PlayCircleOutlined />} loading={starting} onClick={start}>
              Start Migration
            </Button>
            <Button size="large" onClick={() => navigate('/migrations')}>
              Cancel
            </Button>
          </Space>
        </Card>
      </Form>
    </div>
  );
}
