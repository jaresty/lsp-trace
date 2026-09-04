export default class UploadPanel {
  uploadFile(file: File): void {
    void file;
  }

  <template>
    <Uploader @onFileAdded={{this.uploadFile}} />
    <p>{{@data.length}}</p>
  </template>
}
