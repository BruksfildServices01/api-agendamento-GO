let loadingCount = 0;

export function showLoading() {
  loadingCount++;
  document.body.classList.add('is-loading');
}

export function hideLoading() {
  loadingCount--;
  if (loadingCount <= 0) {
    loadingCount = 0;
    document.body.classList.remove('is-loading');
  }
}
