import Model from '@warp-drive/legacy/model';

class UserImportModel extends Model {}

class UploadsServiceSynthetic {
  uploads: UserImportModel[] = [];

  async poll(): Promise<void> {
    for (const upload of this.uploads) {
      await upload.reload();
    }
  }
}

void UploadsServiceSynthetic;
