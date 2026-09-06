import { task } from 'ember-concurrency';
import Model from '@warp-drive/legacy';

const typedTask = task(async () => 'typed');
typedTask.perform();

/** @type {import('ember-concurrency').Task<string, []>} */
const documentedTask = typedTask;
documentedTask.perform();

/** @type {Model} */
const documentedModel = new Model();
documentedModel.reload();

const unrelatedTask = { perform() { return 'control'; } };
unrelatedTask.perform();
const unrelatedModel = { reload() { return 'control'; } };
unrelatedModel.reload();
