import React from 'react';
import {
  Button,
  Card,
  Empty,
  Popconfirm,
  Space,
  Table,
  Typography,
  App,
} from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { Link, useNavigate } from 'react-router-dom';
import { apiDelete, apiGet, apiPost } from '../api/client';
import type { Connection, ConnectionTestResult } from '../types';
import { ConnTypeTag } from '../components/ConnBadge';
import { formatTime } from '../utils/format';

export default function Connections() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [items, setItems] = React.useState<Connection[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [testingId, setTestingId] = React.useState<number | null>(null);

  const load = React.useCallback(() => {
    setLoading(true);
    apiGet<Connection[]>('/connections')
      .then(setItems)
      .catch((e: Error) => message.error(e.message))
      .finally(() => setLoading(false));
  }, [message]);

  React.useEffect(load, [load]);

  const test = async (conn: Connection) => {
    setTestingId(conn.id);
    try {
      const r = await apiPost<ConnectionTestResult>(`/connections/${conn.id}/test`);
      message.success(`Connected to ${r.version} (${r.latency_ms} ms)`);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setTestingId(null);
    }
  };

  const remove = async (id: number) => {
    try {
      await apiDelete(`/connections/${id}`);
      message.success('Connection deleted');
      load();
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const columns = [
    { title: 'Name', dataIndex: 'name', render: (v: string, r: Connection) => <Link to={`/connections/${r.id}/edit`}>{v}</Link> },
    { title: 'Type', dataIndex: 'type', render: (v: Connection['type']) => <ConnTypeTag type={v} /> },
    { title: 'Host', dataIndex: 'host' },
    { title: 'Port', dataIndex: 'port', width: 90 },
    { title: 'Database', dataIndex: 'database' },
    { title: 'Updated', dataIndex: 'updated_at', render: (v: string) => formatTime(v) },
    {
      title: 'Actions',
      width: 220,
      render: (_: unknown, r: Connection) => (
        <Space>
          <Button
            size="small"
            icon={<ThunderboltOutlined />}
            loading={testingId === r.id}
            onClick={() => test(r)}
          >
            Test
          </Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => navigate(`/connections/${r.id}/edit`)}>
            Edit
          </Button>
          <Popconfirm title="Delete this connection?" onConfirm={() => remove(r.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Card
      title="Connections"
      extra={
        <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/connections/new')}>
          New Connection
        </Button>
      }
    >
      <Table
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={items}
        locale={{ emptyText: <Empty description="No connections yet" /> }}
      />
      <Typography.Paragraph type="secondary" style={{ marginTop: 12, fontSize: 12 }}>
        Passwords are encrypted at rest and never returned by the API.
      </Typography.Paragraph>
    </Card>
  );
}
