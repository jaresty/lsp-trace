import Component from '@glimmer/component';

interface UploadPanelSignature {
  Args: {
    data: string[];
  };
}

export default class UploadPanel extends Component<UploadPanelSignature> {
  get itemCount(): number {
    return this.args.data.length;
  }

  <template>
    <p>{{this.itemCount}} {{@data.length}}</p>
  </template>
}
