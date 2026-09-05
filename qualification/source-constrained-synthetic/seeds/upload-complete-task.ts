import type { TaskForAsyncTaskFunction } from 'ember-concurrency';

// Source import: import { keepLatestTask } from 'ember-concurrency';
// Source initializer: uploadComplete = keepLatestTask(async () => { ... })
class UploadsServiceSynthetic {
  declare uploadComplete: TaskForAsyncTaskFunction<object, () => Promise<void>>;

  pollCompletion(): void {
    this.uploadComplete.perform();
  }
}

void UploadsServiceSynthetic;
