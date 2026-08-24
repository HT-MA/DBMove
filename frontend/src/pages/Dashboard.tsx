import React from 'react';
import { Card, Col, Empty, Row, Statistic, Table, Typography } from 'antd';
import { Link } from 'react-router-dom';
import { apiGet } from '../api/client';
import type { MigrationTask, Stats } from '../types';
import StatusTag from '../components/StatusTag';
import { ConnectionSummary } from '../components/ConnBadge';

export default function Dashboard() {
  const [stats, setStats] = React.useState<Stats | null>(null);
  const [loading, setLoading] = React.useState(true);

  React.useEffect(() => {
    apiGet<Stats>('/stats')
      .then(setStats)
      .finally(() => setLoading(false));
  }, []);

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 80,
      render: (id: number) => <Link to={`/migrations/${id}`}>#{id}</Link>,
    },
    { title: 'Name', dataIndex: 'name' },
    {
      title: 'Source',
      render: (_: unknown, r: MigrationTask) => (
        <ConnectionSummary
          type={r.source_connection?.type || 'mysql'}
          name={r.source_connection?.name || '-'}
          database={r.source_database}
        />
      ),
    },
    {
      title: 'Target',
      render: (_: unknown, r: MigrationTask) => (
        <ConnectionSummary
          type={r.target_connection?.type || 'mysql'}
          name={r.target_connection?.name || '-'}
          database={r.target_database}
        />
      ),
    },
    { title: 'Status', dataIndex: 'status', render: (s: MigrationTask['status']) => <StatusTag status={s} /> },
  ];

  const c = stats?.connections;
  const m = stats?.migrations;

  return (
    <div>
      <Typography.Title level={4}>Dashboard</Typography.Title>
      <Row gutter={[16, 16]}>
        <Col xs={12} md={6}>
          <Card loading={loading} className="stat-card">
            <Statistic title="Connections" value={c?.total ?? 0} />
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card loading={loading} className="stat-card">
            <Statistic title="MySQL" value={c?.mysql ?? 0} valueStyle={{ color: '#1677ff' }} />
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card loading={loading} className="stat-card">
            <Statistic title="PostgreSQL" value={c?.postgresql ?? 0} valueStyle={{ color: '#13c2c2' }} />
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card loading={loading} className="stat-card">
            <Statistic title="Migrations (total)" value={m?.total ?? 0} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={12} md={6}>
          <Card className="stat-card">
            <Statistic title="Running" value={m?.running ?? 0} valueStyle={{ color: '#1677ff' }} />
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card className="stat-card">
            <Statistic title="Success" value={m?.success ?? 0} valueStyle={{ color: '#52c41a' }} />
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card className="stat-card">
            <Statistic title="Failed" value={m?.failed ?? 0} valueStyle={{ color: '#ff4d4f' }} />
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card className="stat-card">
            <Statistic title="Pending / Cancelled" value={(m?.pending ?? 0) + (m?.cancelled ?? 0)} />
          </Card>
        </Col>
      </Row>

      <Card title="Recent Migrations" style={{ marginTop: 16 }}>
        <Table
          rowKey="id"
          size="small"
          loading={loading}
          columns={columns}
          dataSource={stats?.recent_migrations || []}
          pagination={false}
          locale={{ emptyText: <Empty description="No migrations yet" /> }}
        />
      </Card>
    </div>
  );
}
