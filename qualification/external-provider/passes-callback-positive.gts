type Callback = (value: string) => void;

function receive(callback: Callback): void {
  void callback;
}

function format(value: string): void {
  void value;
}

receive(format);
