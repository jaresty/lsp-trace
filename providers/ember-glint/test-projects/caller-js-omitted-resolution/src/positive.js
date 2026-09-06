import { task } from 'ember-concurrency';

const callerOwnedTask = task(async () => 'caller-owned');
callerOwnedTask.perform();
