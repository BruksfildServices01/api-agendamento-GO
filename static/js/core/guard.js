import { isAuthenticated } from './auth.js';

export function requireAuth() {
  if (!isAuthenticated()) {
    window.location.href = '/web/app/login';
  }
}
