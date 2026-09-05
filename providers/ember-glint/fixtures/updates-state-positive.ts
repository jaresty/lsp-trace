class CounterPanel {
  count = 0;
  enabled = false;

  apply(): void {
    this.enabled = true;
    this.count++;
  }
}
