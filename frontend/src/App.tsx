import type { Component, ParentComponent } from 'solid-js';
import { Show } from 'solid-js';
import { Router, Route, Navigate } from '@solidjs/router';
import { AuthProvider, useAuth } from './lib/auth';
import { useTheme } from './lib/theme';
import Layout from './components/Layout';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Devices from './pages/Devices';
import DeviceDetail from './pages/DeviceDetail';
import Faults from './pages/Faults';
import Firmwares from './pages/Firmwares';
import Settings from './pages/Settings';

// Initialize theme on app load
useTheme();

const ProtectedLayout: ParentComponent = (props) => {
  const { isAuthenticated } = useAuth();

  return (
    <Show when={isAuthenticated()} fallback={<Navigate href="/login" />}>
      <Layout>{props.children}</Layout>
    </Show>
  );
};

const App: Component = () => {
  return (
    <AuthProvider>
      <Router>
        <Route path="/login" component={Login} />
        <Route path="/" component={ProtectedLayout}>
          <Route path="/" component={Dashboard} />
          <Route path="/devices" component={Devices} />
          <Route path="/device/:serial" component={DeviceDetail} />
          <Route path="/faults" component={Faults} />
          <Route path="/firmwares" component={Firmwares} />
          <Route path="/settings" component={Settings} />
        </Route>
      </Router>
    </AuthProvider>
  );
};

export default App;
