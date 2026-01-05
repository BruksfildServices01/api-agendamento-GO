import { requireAuth } from '../core/guard.js';
import { initDashboard } from '../modules/dashboard.js';
import { logout } from '../core/auth.js';

requireAuth();

initDashboard();

document.getElementById('logoutBtn')
  ?.addEventListener('click', logout);
