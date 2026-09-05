import Component from '@glimmer/component';
import { task } from 'ember-concurrency';

export default class UploadComponent extends Component {
  upload = task(async () => {
    return 'uploaded';
  });

  startUpload(): void {
    this.upload.perform();
  }
}
