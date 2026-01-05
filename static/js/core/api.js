import { getToken, logout } from './auth.js';
import { showLoading, hideLoading } from './loading.js';

const API_BASE = '/api';

export async function apiRequest(path, options = {}) {
  showLoading();

  try {
    const headers = {
      'Content-Type': 'application/json',
      ...(options.headers || {})
    };

    const token = getToken();
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }

    const res = await fetch(API_BASE + path, {
      ...options,
      headers
    });

    if (res.status === 401) {
      logout();
      throw new Error('Sessão expirada');
    }

    if (!res.ok) {
      let err = { message: 'Erro inesperado' };
      try { err = await res.json(); } catch (_) {}
      throw err;
    }

    if (res.status === 204) return null;

    return res.json();
  } finally {
    hideLoading();
  }
}
