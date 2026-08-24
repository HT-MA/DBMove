import React from 'react';
import {
  App,
  Button,
  Card,
  Empty,
  Progress,
  Select,
  Space,
  Table,
} from 'antd';
import { Link, useNavigate } from 'react-router-dom';
import { PlusOutlined } from '@ant-design/icons';
import { apiGet } from '../api/client';
import type { MigrationTask, PageResult, TaskStatus } from '../types';
import StatusTag from '../components/StatusTag';
import { ConnectionSummary, DatabaseMappingSummary } from '../components/ConnBadge';
import { formatTime } from '../utils/format';

export default function Migrations() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [items, setItems] = React.useState<MigrationTask[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(20);
  const [status, setStatus] = React.useState<string>();
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(() => {
    setLoading(true);
    apiGet<PageResult<MigrationTask>>('/migrations', {
      page,
      page_size: pageSize,
      status: status || undefined,
    })
      .then((r) => {
        setItems(r.items);
        setTotal(r.total);
      })
      .catch((e: Error) => message.error(e.message))
      .finally(() => setLoading(false));
  }, [page, pageSize, status, message]);

  React.useEffect(load, [load]);

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 80, render: (v: number) => <Link to={`/migrations/${v}`}>#{v}</Link> },
    { title: 'Name', dataIndex: 'name', render: (v: string, r: MigrationTask) => <Link to={`/migrations/${r.id}`}>{v}</Link> },
    {
      title: 'Source',
      render: (_: unknown, r: MigrationTask) => (
        <div>
          <ConnectionSummary type={r.source_connection?.type || 'mysql'} name={r.source_connection?.name || '-'} />
          <DatabaseMappingSummary pairs={r.databases?.length ? r.databases : [{ source: r.source_database, target: r.target_database }]} />
        </div>
      ),
    },
    {
      title: 'Target',
      render: (_: unknown, r: MigrationTask) => (
        <ConnectionSummary type={r.target_connection?.type || 'mysql'} name={r.target_connection?.name || '-'} />
      ),
    },
    { title: 'Status', dataIndex: 'status', render: (s: TaskStatus) => <StatusTag status={s} /> },
    {
      title: 'Progress',
      dataIndex: 'progress',
      width: 140,
      render: (p: number) => <Progress percent={p} size="small" status={p >= 100 ? 'success' : 'active'} />,
    },
    { title: 'Created', dataIndex: 'created_at', render: (v: string) => formatTime(v) },
  ];

  return (
    <Card
      title="Migrations"
      extra={
        <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/migrations/new')}>
          New Migration
        </Button>
      }
    >
      <Space style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder="Filter by status"
          style={{ width: 180 }}
          value={status}
          onChange={(v) => {
            setStatus(v);
            setPage(1);
          }}
          options={[
            { value: 'PENDING', label: 'Pending' },
            { value: 'PREPARING', label: 'Preparing' },
            { value: 'RUNNING', label: 'Running' },
            { value: 'SUCCESS', label: 'Success' },
            { value: 'FAILED', label: 'Failed' },
            { value: 'CANCELLED', label: 'Cancelled' },
          ]}
        />
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={items}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
          },
        }}
        locale={{ emptyText: <Empty description="No migrations yet" /> }}
      />
    </Card>
  );
}
