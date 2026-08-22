import { buildTaskWebUrl } from '../task-links'

test('builds an encoded canonical web task route', () => {
  const url = new URL(buildTaskWebUrl('Customer Success', 'task/id'))
  expect(url.pathname).toBe('/w/Customer%20Success/pm/tasks/task%2Fid')
})
