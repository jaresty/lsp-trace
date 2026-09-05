import Service, { service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import {
  task,
  restartableTask,
  keepLatestTask,
  timeout,
} from 'ember-concurrency';

import { errorMessages, successMessages } from '../constants';
import { ERROR_TYPES, IMPORT_STATUSES } from '../models/user-import';

const POLL_INTERVAL = 1000;
export default class UploadsService extends Service {
  @service store;

  @service data;

  @service network;

  @service log;

  @service notifications;

  @service metrics;

  @tracked
  uploads = [];

  addInProgressUploads(uploads) {
    this.uploads = [...this.uploads, ...uploads];
    this.poll.perform();
  }

  uploadDataset = task(async (file) => {
    let dataset = this.store.createRecord('dataset', {
      name: file.name,
      uploadProcessed: false,
    });
    await dataset.save();
    await this.uploadFile(
      file,
      dataset.directUploadCredentials.url,
      dataset.directUploadCredentials.credentials,
    );

    await this.network.request(
      `/api/v1/datasets/${dataset.id}/upload_finished`,
      {
        method: 'POST',
      },
    );
    return dataset;
  });

  uploadDatasetAndPollForStatus = task(async (file) => {
    try {
      let userImport = this.store.createRecord('user-import', {
        kind: 'upload',
        source: file.name,
        status: IMPORT_STATUSES.pending,
      });
      await userImport.save();

      await this.uploadFile(
        file,
        userImport.uploadCredentials.url,
        userImport.uploadCredentials.credentials,
      );

      let dataset = this.store.createRecord('dataset', {
        name: file.name,
        uploadProcessed: false,
        userImport,
      });
      await dataset.save();

      this.uploads = [...this.uploads, userImport];
      this.poll.perform();
    } catch (e) {
      this.log.error(e);
      this.notifications.error(errorMessages.addUpload);
    }
  });

  poll = restartableTask(async () => {
    while (this.uploads.length > 0) {
      try {
        for (let upload of this.uploads) {
          await upload.reload();
          if (upload.isUploadComplete) {
            this.uploads = this.uploads.filter((u) => u.id !== upload.id);
            this.uploadComplete.perform();
            if (upload.isSuccessfulUpload) {
              this.notifications.success(successMessages.addUpload);
            } else {
              this.notifications.error(errorMessageForImport(upload));
            }
          }
        }
        await timeout(POLL_INTERVAL);
      } catch (e) {
        this.log.error(e);
      }
    }
  });

  uploadAdmissionsListAndPollForStatus = task(async (kind, year, file) => {
    let userImport = await this.uploadToAdmissionsList.perform(
      kind,
      year,
      file,
    );
    if (userImport) {
      this.uploads = [...this.uploads, userImport];
      this.poll.perform();
    }
  });

  uploadToAdmissionsList = task(async (kind, year, file) => {
    try {
      this.metrics.trackEvent({
        category: 'Your Data',
        action: 'Upload Admissions List',
        label: kind,
        value: year,
      });

      let userImport = this.store.createRecord('user-import', {
        kind: 'upload',
        source: file.name,
        status: IMPORT_STATUSES.pending,
      });
      await userImport.save();

      await this.uploadFile(
        file,
        userImport.uploadCredentials.url,
        userImport.uploadCredentials.credentials,
      );

      await this.saveImportToAdmissionsList.perform(kind, year, userImport);
      return userImport;
    } catch (e) {
      this.log.error(e);
      this.notifications.error(errorMessages.addUpload);
    }
  });

  uploadComplete = keepLatestTask(async () => {
    await this.data.load.perform();
  });

  saveImportToAdmissionsList = task(
    { enqueue: true },
    async (kind, year, userImport) => {
      let admissionsList = await this.findOrCreateAdmissionsList(kind, year);
      admissionsList.userImports.push(userImport);

      await admissionsList.save();
      await this.data.load.perform();
    },
  );

  async findOrCreateAdmissionsList(kind, year) {
    let lists = await this.store.query('admissions-list', {
      filter: {
        kind,
        year,
      },
    });

    if (lists.length) {
      return lists[0];
    }
    return this.store.createRecord('admissions-list', { kind, year });
  }

  async uploadFile(file, url, data, contentType = 'text/csv') {
    await file.upload(url, {
      data,
      contentType,
    });
  }
}

function errorMessageForImport(userImport) {
  switch (userImport.errorType) {
    case ERROR_TYPES.badFileFormat:
      return errorMessages.addUploadBadFileFormat;
    case ERROR_TYPES.badFileType:
      return errorMessages.addUploadBadFileType;
    case ERROR_TYPES.unknown:
    case ERROR_TYPES.downloadError:
    default:
      return errorMessages.addUpload;
  }
}
