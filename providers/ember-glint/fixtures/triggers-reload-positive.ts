import Model from '@ember-data/model';

export default class Person extends Model {}

export async function refreshPerson(person: Person): Promise<Person> {
  return person.reload();
}
