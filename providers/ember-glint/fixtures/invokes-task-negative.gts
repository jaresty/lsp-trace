import Component from '@glimmer/component';

class ReportRunner {
  perform(): string {
    return 'ordinary method';
  }
}

export default class ReportComponent extends Component {
  runner = new ReportRunner();

  runReport(): void {
    this.runner.perform();
  }
}
