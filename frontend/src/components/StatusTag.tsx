import { Tag } from 'antd';
import type { TaskStatus } from '../types';

const statusMap: Record<TaskStatus, { color: string; label: string }> = {
  PENDING: { color: 'default', label: 'Pending' },
  PREPARING: { color: 'processing', label: 'Preparing' },
  RUNNING: { color: 'processing', label: 'Running' },
  SUCCESS: { color: 'success', label: 'Success' },
  FAILED: { color: 'error', label: 'Failed' },
  CANCELLED: { color: 'warning', label: 'Cancelled' },
};

export default function StatusTag({ status }: { status: TaskStatus }) {
  const s = statusMap[status] || { color: 'default', label: status };
  return <Tag color={s.color}>{s.label}</Tag>;
}
