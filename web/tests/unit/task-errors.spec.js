import { expect } from 'chai';
import taskErrors from '@/lib/taskErrors';

const records = (...lines) => lines.map((output) => ({ output }));

describe('Task log error extraction', () => {
  it('extracts ANSI-colored errors and continuation lines without unrelated output', () => {
    const error = taskErrors(records(
      'Starting task',
      '[WARNING]: error contacting optional endpoint',
      '\u001b[31m[ERROR]: Cannot call Galaxy\u001b[0m',
      'https://example.test/api: <urlopen error',
      '[Errno 101] Network unreachable>',
      '',
      '',
      'Cleaning workspace',
    ));
    expect(error.matched).to.equal(true);
    expect(error.text).to.equal('[ERROR]: Cannot call Galaxy\nhttps://example.test/api: <urlopen error\n[Errno 101] Network unreachable>');
  });

  it('includes Ansible fatal and Terraform error blocks, removes duplicate errors', () => {
    const result = taskErrors(records(
      'fatal: [server]: FAILED! => {"msg":"denied"}',
      'PLAY RECAP',
      '│ Error: permission denied\n│ on main.tf line 1',
      '',
      '',
      '│ Error: permission denied\n│ on main.tf line 1',
    ));
    expect(result.matched).to.equal(true);
    expect(result.text).to.contain('fatal: [server]');
    expect(result.text).not.to.contain('PLAY RECAP');
    expect(result.text.match(/permission denied/g)).to.have.length(1);
  });

  it('falls back to the last ten nonempty lines, without treating warnings as errors', () => {
    const result = taskErrors(records('[WARNING]: ERROR mentioned in warning', ...Array.from({ length: 12 }, (_, i) => `line ${i}`)));
    expect(result.matched).to.equal(false);
    expect(result.text.split('\n')).to.have.length(10);
    expect(result.text.split('\n')[0]).to.equal('line 2');
    expect(taskErrors([])).to.deep.equal({ matched: false, text: '' });
  });

  it('keeps HTML as literal text and bounds the popup output size', () => {
    expect(taskErrors(records('ERROR: <img src=x onerror=alert(1)>')).text)
      .to.equal('ERROR: <img src=x onerror=alert(1)>');
    expect(taskErrors(records(`ERROR: ${'x'.repeat(30000)}`)).text).to.have.length(20000);
  });
});
