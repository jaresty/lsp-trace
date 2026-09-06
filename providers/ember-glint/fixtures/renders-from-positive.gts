import Component from '@glimmer/component';

interface PanelSignature {
  Args: {
    records: string[];
  };
}

export default class Panel extends Component<PanelSignature> {
  get visibleTotal(): number {
    return this.args.records.length;
  }

  <template>
    <p>{{this.visibleTotal}}</p>
  </template>
}
