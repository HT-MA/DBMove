import React from 'react';
import { Layout, Menu, Switch, Typography, theme } from 'antd';
import {
  DashboardOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  MoonOutlined,
  SettingOutlined,
  SunOutlined,
} from '@ant-design/icons';
import { useLocation, useNavigate } from 'react-router-dom';

const { Sider, Header, Content } = Layout;

const menuItems = [
  { key: '/dashboard', icon: <DashboardOutlined />, label: 'Dashboard' },
  { key: '/connections', icon: <DatabaseOutlined />, label: 'Connections' },
  { key: '/migrations', icon: <DeploymentUnitOutlined />, label: 'Migrations' },
  { key: '/settings', icon: <SettingOutlined />, label: 'Settings' },
];

export default function AppLayout({
  children,
  dark,
  onToggleTheme,
}: {
  children: React.ReactNode;
  dark: boolean;
  onToggleTheme: () => void;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const {
    token: { colorBgContainer },
  } = theme.useToken();

  const selected = menuItems
    .filter((m) => location.pathname.startsWith(m.key))
    .map((m) => m.key);

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider theme={dark ? 'dark' : 'light'} width={220} breakpoint="lg" collapsedWidth={64}>
        <div
          style={{
            height: 56,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontWeight: 700,
            fontSize: 18,
            letterSpacing: 1,
          }}
        >
          DBMove
        </div>
        <Menu
          theme={dark ? 'dark' : 'light'}
          mode="inline"
          selectedKeys={selected}
          items={menuItems}
          onClick={(e) => navigate(e.key)}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            background: colorBgContainer,
            padding: '0 24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            borderBottom: '1px solid rgba(5,5,5,0.06)',
          }}
        >
          <Typography.Text type="secondary" style={{ fontSize: 13 }}>
            Database Migration Platform
          </Typography.Text>
          <Switch
            checkedChildren={<MoonOutlined />}
            unCheckedChildren={<SunOutlined />}
            checked={dark}
            onChange={onToggleTheme}
          />
        </Header>
        <Content style={{ margin: 24 }}>{children}</Content>
      </Layout>
    </Layout>
  );
}
