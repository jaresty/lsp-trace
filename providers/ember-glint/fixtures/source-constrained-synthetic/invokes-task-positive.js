import { task } from 'ember-concurrency';

const upload = task(async () => 'uploaded');
export function invoke() {
  return upload.perform();
}
