import { apiRequest } from '../core/api.js';
import { logout } from '../core/auth.js';
import { getErrorMessage } from '../core/error.js';

export async function initDashboard() {
  const userNameEl = document.getElementById('userName');
  const shopNameEl = document.getElementById('shopName');
  const errorBox = document.getElementById('dashboardError');

  try {
    const data = await apiRequest('/me');

    userNameEl.textContent = data.user.name;
    shopNameEl.textContent = data.barbershop.name;

  } catch (err) {
    errorBox.textContent = getErrorMessage(err);
    errorBox.classList.remove('hidden');
  }

  bindActions();
}

function bindActions() {
  document.querySelectorAll('[data-action]').forEach(btn => {
    btn.addEventListener('click', () => {
      const action = btn.dataset.action;

      switch (action) {
        case 'services':
          window.location.href = '/web/app/services';
          break;

        case 'agenda':
          alert('Agenda — em breve (modal)');
          break;

        case 'hours':
          alert('Horários — em breve (modal)');
          break;

        case 'logout':
          logout();
          break;
      }
    });
  });
}
