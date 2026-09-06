/** @param {import('@ember-data/model').default} person */
export function refresh(person) {
  return person.reload();
}
