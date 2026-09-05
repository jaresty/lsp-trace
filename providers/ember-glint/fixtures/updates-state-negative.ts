class ForeignState {
  count = 0;
}

class CounterPanel {
  count = 0;

  apply(other: ForeignState): void {
    let count = 0;
    count = 1;
    this.missing = 2;
    other.count = 3;
  }
}
