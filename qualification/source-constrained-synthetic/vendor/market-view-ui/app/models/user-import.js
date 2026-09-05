import { capitalize } from '@ember/string';
import Model, { attr, hasMany } from '@warp-drive/legacy/model';
import moment from 'moment';

export const IMPORT_STATUSES = {
  done: 'done',
  inProgress: 'in-progress',
  pending: 'pending',
  error: 'error',
};

export const ERROR_TYPES = {
  downloadError: 'download-error',
  badFileFormat: 'bad-file-format',
  badFileType: 'bad-file-type',
  unknown: 'unknown-error',
  missingApiScope: 'missing-api-scope',
  badClientCredentials: 'bad-client-credentials',
};

export default class UserImportModel extends Model {
  @attr('string')
  kind;

  @attr('string')
  source;

  @attr('string')
  status;

  @attr('number')
  count;

  @attr('string')
  errorType;

  @attr('date')
  finishedAt;

  @attr('object')
  uploadCredentials;

  @hasMany('import-failure', { async: false, inverse: 'userImport' })
  failures;

  @hasMany('admissions-list-item', { async: false, inverse: 'userImport' })
  admissionsListItems;

  get name() {
    return this.source;
  }

  get sourceName() {
    if (this.isDASL) {
      return 'DASL';
    }
    return capitalize(this.source);
  }

  set name(newValue) {
    this.source = newValue;
  }

  get uploadedAt() {
    return this.finishedAt;
  }

  get isUpload() {
    return this.kind === 'upload';
  }

  get isIntegration() {
    return this.kind === 'integration';
  }

  get isDASL() {
    return this.isIntegration && this.source === 'dasl';
  }

  get isBlackbaud() {
    return this.isIntegration && this.source === 'blackbaud';
  }

  get isVeracross() {
    return this.isIntegration && this.source === 'veracross';
  }

  get isSuccessful() {
    return this.status === IMPORT_STATUSES.done;
  }

  get isComplete() {
    return (
      this.status === IMPORT_STATUSES.done ||
      this.status === IMPORT_STATUSES.error
    );
  }

  get isSuccessfulUpload() {
    return this.isUpload && this.isSuccessful;
  }

  get isSuccessfulIntegration() {
    return this.isIntegration && this.isSuccessful;
  }

  get isUploadInProgress() {
    return (
      this.isUpload &&
      (this.status === IMPORT_STATUSES.inProgress ||
        this.status === IMPORT_STATUSES.pending)
    );
  }

  get isUploadComplete() {
    return this.isUpload && this.isComplete;
  }

  get hasFailures() {
    return this.failures.length > 0;
  }

  get isRecent() {
    return this.finishedAt > moment().subtract(1, 'day');
  }
}
