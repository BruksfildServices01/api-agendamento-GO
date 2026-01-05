export function getErrorMessage(err) {
  if (!err) return 'Erro inesperado';

  if (typeof err === 'string') return err;

  if (err.message) return err.message;

  if (err.error) return err.error;

  return 'Erro inesperado';
}
