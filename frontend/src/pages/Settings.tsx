import React from 'react';
import { Card, Descriptions, Typography } from 'antd';
import { apiGet } from '../api/client';
import type { AppInfo } from '../types';

export default function Settings() {
  const [info, setInfo] = React.useState<AppInfo | null>(null);

  React.useEffect(() => {
    apiGet<AppInfo>('/info').then(setInfo).catch(() => undefined);
  }, []);

  return (
    <div>
      <Typography.Title level={4}>Settings</Typography.Title>
      <Card title="About DBMove">
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="Name">{info?.name || 'DBMove'}</Descriptions.Item>
          <Descriptions.Item label="Version">{info?.version || '-'}</Descriptions.Item>
          <Descriptions.Item label="Execution Mode">{info?.execution_mode || '-'}</Descriptions.Item>
          <Descriptions.Item label="Max Concurrent Migrations">{info?.max_concurrent_migrations ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="Supported Databases">
            {info?.supported_databases?.join(', ') || 'mysql, postgresql'}
          </Descriptions.Item>
          <Descriptions.Item label="Supported Migration Types">
            {info?.supported_migration_types?.join(', ') || 'FULL'}
          </Descriptions.Item>
        </Descriptions>
      </Card>
    </div>
  );
}
