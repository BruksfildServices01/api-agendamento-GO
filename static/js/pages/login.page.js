import { apiRequest } from '../core/api.js';
import { saveToken, isAuthenticated } from '../core/auth.js';

if (isAuthenticated()) {
  window.location.href = '/web/app/dashboard';
}

const form = document.getElementById('loginForm');
const errorBox = document.getElementById('loginError');

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  errorBox.classList.add('hidden');

  const formData = new FormData(form);

  try {
    const res = await apiRequest('/auth/login', {

      method: 'POST',
      body: JSON.stringify({
        email: formData.get('email'),
        password: formData.get('password')
      })
    });

    saveToken(res.token);
    window.location.href = '/web/app/dashboard';

  } catch (err) {
    errorBox.textContent = 'Email ou senha inválidos.';
    errorBox.classList.remove('hidden');
  }
});
