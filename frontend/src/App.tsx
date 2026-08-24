import { Navigate, Route, Routes } from 'react-router-dom';
import AppLayout from './components/AppLayout';
import Dashboard from './pages/Dashboard';
import Connections from './pages/Connections';
import ConnectionFormPage from './pages/ConnectionFormPage';
import Migrations from './pages/Migrations';
import NewMigration from './pages/NewMigration';
import MigrationDetail from './pages/MigrationDetail';
import Settings from './pages/Settings';

export default function App({ dark, onToggleTheme }: { dark: boolean; onToggleTheme: () => void }) {
  return (
    <AppLayout dark={dark} onToggleTheme={onToggleTheme}>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/connections" element={<Connections />} />
        <Route path="/connections/new" element={<ConnectionFormPage />} />
        <Route path="/connections/:id/edit" element={<ConnectionFormPage />} />
        <Route path="/migrations" element={<Migrations />} />
        <Route path="/migrations/new" element={<NewMigration />} />
        <Route path="/migrations/:id" element={<MigrationDetail />} />
        <Route path="/settings" element={<Settings />} />
      </Routes>
    </AppLayout>
  );
}
