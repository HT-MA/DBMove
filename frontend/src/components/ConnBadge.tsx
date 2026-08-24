import { DatabaseOutlined } from '@ant-design/icons';
import { Space, Tag } from 'antd';
import type { ConnType, DatabasePair } from '../types';

const typeColors: Record<ConnType, string> = {
  mysql: 'geekblue',
  postgresql: 'cyan',
  dm8: 'purple',
  redis: 'volcano',
};

export function ConnTypeTag({ type }: { type: ConnType }) {
  return <Tag color={typeColors[type] || 'default'}>{type}</Tag>;
}

export function ConnectionSummary({ type, name, database }: { type: ConnType; name: string; database?: string }) {
  return (
    <Space size={4} wrap>
      <DatabaseOutlined />
      <ConnTypeTag type={type} />
      <span>{name}</span>
      {database ? <Tag>{database}</Tag> : null}
    </Space>
  );
}

export function DatabaseMappingSummary({ pairs }: { pairs: DatabasePair[] }) {
  const shown = pairs.slice(0, 3);
  const extra = pairs.length - shown.length;
  return (
    <Space size={4} wrap>
      {shown.map((p, i) => (
        <Tag key={`${p.source}-${i}`}>
          {p.source} → {p.target}
        </Tag>
      ))}
      {extra > 0 ? <Tag>+{extra} more</Tag> : null}
    </Space>
  );
}
