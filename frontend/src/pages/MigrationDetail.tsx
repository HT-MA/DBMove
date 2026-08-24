import React from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Progress,
  Row,
  Space,
  Switch,
  Typography,
} from 'antd';
import {
  ArrowLeftOutlined,
  ArrowRightOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ReloadOutlined,
  StopOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { apiGet, apiPost } from '../api/client';
import type { MigrationLog, MigrationTask, TaskStatus } from '../types';
import StatusTag from '../components/StatusTag';
import { ConnectionSummary, DatabaseMappingSummary } from '../components/ConnBadge';
import { formatBytes, formatDuration, formatSpeed, formatTime } from '../utils/format';

export default function MigrationDetail() {
  const { id } = useParams();
  const taskId = Number(id);
  const navigate = useNavigate();
  const { message } = App.useApp();
  const [task, setTask] = React.useState<MigrationTask | null>(null);
  const [logs, setLogs] = React.useState<MigrationLog[]>([]);
  const [autoScroll, setAutoScroll] = React.useState(true);
  const [acting, setActing] = React.useState(false);
  const [polling, setPolling] = React.useState(true);
  const logRef = React.useRef<HTMLDivElement>(null);

  const loadTask = React.useCallback(() => {
    apiGet<MigrationTask>(`/migrations/${taskId}`)
      .then(setTask)
      .catch((e: Error) => message.error(e.message));
  }, [taskId, message]);

  React.useEffect(() => {
    if (!polling) return;
    loadTask();
    const timer = window.setInterval(loadTask, 2000);
    return () => window.clearInterval(timer);
  }, [loadTask, polling]);

  React.useEffect(() => {
    if (
      task &&
      (task.status === 'SUCCESS' || task.status === 'FAILED' || task.status === 'CANCELLED')
    ) {
      setPolling(false);
    }
  }, [task]);

  React.useEffect(() => {
    const es = new EventSource(`/api/v1/migrations/${taskId}/logs/stream`);
    es.addEventListener('log', (e) => {
      const data = JSON.parse((e as MessageEvent).data) as MigrationLog;
      setLogs((prev) => [...prev.slice(-1499), data]);
    });
    es.addEventListener('status', (e) => {
      const data = JSON.parse((e as MessageEvent).data) as { status: TaskStatus; error_message?: string };
      setTask((prev) => (prev ? { ...prev, status: data.status, error_message: data.error_message } : prev));
      if (data.status === 'SUCCESS') message.success('Migration completed');
      if (data.status === 'FAILED') message.error('Migration failed');
    });
    es.onerror = () => {
      /* EventSource reconnects automatically */
    };
    return () => es.close();
  }, [taskId, message]);

  React.useEffect(() => {
    if (autoScroll && logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const act = async (action: 'start' | 'cancel' | 'retry') => {
    setActing(true);
    try {
      await apiPost(`/migrations/${taskId}/${action}`);
      message.success(`Task ${action} requested`);
      loadTask();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setActing(false);
    }
  };

  if (!task) {
    return <Card loading />;
  }

  const active = task.status === 'PREPARING' || task.status === 'RUNNING';
  const canStart = task.status === 'PENDING' || task.status === 'CANCELLED';
  const canCancel = task.status === 'PENDING' || active;

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/migrations')}>
          Back
        </Button>
        <Typography.Title level={4} style={{ margin: 0 }}>
          Migration #{task.id}
        </Typography.Title>
        <StatusTag status={task.status} />
      </Space>

      <Card>
        <div className="migration-flow" style={{ marginBottom: 16 }}>
          <ConnectionSummary
            type={task.source_connection?.type || 'mysql'}
            name={task.source_connection?.name || '-'}
          />
          <span className="flow-arrow">→</span>
          <ConnectionSummary
            type={task.target_connection?.type || 'mysql'}
            name={task.target_connection?.name || '-'}
          />
        </div>
        <DatabaseMappingSummary
          pairs={task.databases?.length ? task.databases : [{ source: task.source_database, target: task.target_database }]}
        />

        <Progress
          percent={task.progress}
          status={task.status === 'FAILED' || task.status === 'CANCELLED' ? 'exception' : task.progress >= 100 ? 'success' : 'active'}
        />

        <Row gutter={[16, 16]} style={{ marginTop: 8 }}>
          <Col xs={24} md={8}>
            <Card size="small" className="stat-card">
              <Descriptions column={1} size="small" title="Overview">
                <Descriptions.Item label="Status">
                  <StatusTag status={task.status} />
                </Descriptions.Item>
                <Descriptions.Item label="Duration">{formatDuration(task.started_at, task.finished_at)}</Descriptions.Item>
                <Descriptions.Item label="Engine">{task.engine}</Descriptions.Item>
                <Descriptions.Item label="Created">{formatTime(task.created_at)}</Descriptions.Item>
              </Descriptions>
            </Card>
          </Col>
          <Col xs={24} md={8}>
            <Card size="small" className="stat-card">
              <Descriptions column={1} size="small" title="Data">
                <Descriptions.Item label="Tables">{`${task.tables_completed} / ${task.tables_total}`}</Descriptions.Item>
                <Descriptions.Item label="Rows">{`${task.rows_completed.toLocaleString()} / ${task.rows_total.toLocaleString()}`}</Descriptions.Item>
                <Descriptions.Item label="Data Size">{formatBytes(task.bytes_transferred)}</Descriptions.Item>
                <Descriptions.Item label="Speed">{formatSpeed(task.speed)}</Descriptions.Item>
              </Descriptions>
            </Card>
          </Col>
          <Col xs={24} md={8}>
            <Card size="small" className="stat-card">
              <Typography.Title level={5} style={{ marginTop: 0 }}>
                Actions
              </Typography.Title>
              <Space wrap>
                {canStart && !task.queued_at ? (
                  <Button type="primary" icon={<CheckCircleOutlined />} loading={acting} onClick={() => act('start')}>
                    Start
                  </Button>
                ) : null}
                {canCancel ? (
                  <Button danger icon={<StopOutlined />} loading={acting} onClick={() => act('cancel')}>
                    Cancel
                  </Button>
                ) : null}
                {task.status === 'FAILED' ? (
                  <Button icon={<ReloadOutlined />} loading={acting} onClick={() => act('retry')}>
                    Retry
                  </Button>
                ) : null}
                {task.status === 'CANCELLED' ? (
                  <Button type="primary" icon={<ReloadOutlined />} loading={acting} onClick={() => act('start')}>
                    Restart
                  </Button>
                ) : null}
              </Space>
              {task.error_message ? (
                <Typography.Paragraph type="danger" style={{ marginTop: 12, marginBottom: 0 }}>
                  <CloseCircleOutlined /> {task.error_message}
                </Typography.Paragraph>
              ) : null}
            </Card>
          </Col>
        </Row>
      </Card>

      <Card
        title="Logs"
        style={{ marginTop: 16 }}
        extra={
          <Space>
            Auto-scroll
            <Switch checked={autoScroll} onChange={setAutoScroll} size="small" />
          </Space>
        }
      >
        <div className="logs-container" ref={logRef}>
          {logs.length === 0 ? (
            <Typography.Paragraph type="secondary" style={{ padding: '0 12px' }}>
              No logs yet.
            </Typography.Paragraph>
          ) : (
            logs.map((l) => (
              <div key={l.id || `${l.created_at}-${logs.indexOf(l)}`} className={`log-line level-${l.level}`}>
                <span style={{ opacity: 0.6, marginRight: 8 }}>{formatTime(l.created_at)}</span>
                <span style={{ fontWeight: 600, marginRight: 8 }}>{l.level}</span>
                {l.message}
              </div>
            ))
          )}
        </div>
      </Card>
    </div>
  );
}
