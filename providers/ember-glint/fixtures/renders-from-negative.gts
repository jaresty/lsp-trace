import Component from '@glimmer/component';

interface PanelSignature {
  Args: {
    visibleTotal: number;
  };
}

export default class Panel extends Component<PanelSignature> {
  visibleTotal: number = 7;

  <template>
    <p>{{this.visibleTotal}}</p>
  </template>
}
