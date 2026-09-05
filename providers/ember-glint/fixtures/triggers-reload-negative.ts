class LocalCache {
  reload(): Promise<LocalCache> {
    return Promise.resolve(this);
  }
}

export async function refreshCache(cache: LocalCache): Promise<LocalCache> {
  return cache.reload();
}
