// Keep diagnostics as plain text: log contents must never become popup HTML.
export default function taskErrors(records = []) {
  const lines = records.flatMap((record) => String(record.output || '')
    // eslint-disable-next-line no-control-regex
    .replace(/\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))/g, '')
    .replace(/\r/g, '').split('\n'));
  const marker = /^\s*[│╷╵]?\s*(?:\[ERROR\]|ERROR(?:[:!\s]|$)|FATAL(?:[:\s]|$)|FAILED(?:!|\s+to\b)|Traceback \(most recent call last\)|[^\s:]+(?:Error|Exception):)/i;
  const blocks = [];
  for (let index = 0; index < lines.length; index += 1) {
    if (marker.test(lines[index])) {
      const block = [lines[index]];
      // Commands often emit multi-line errors as separate log records.
      while (index + 1 < lines.length && block.length < 16) {
        const next = lines[index + 1];
        if (marker.test(next) || /^\s*(?:TASK \[|PLAY \[|PLAY RECAP|\[WARNING\]|[A-Z][a-z]+ing\b)/.test(next)) break;
        if (!next.trim() && !lines[index + 2]?.trim()) break;
        block.push(next);
        index += 1;
      }
      blocks.push(block.join('\n').trim());
    }
  }
  const matched = blocks.length > 0;
  const text = (matched ? [...new Set(blocks)].join('\n\n') : lines.filter((line) => line.trim()).slice(-10).join('\n'));
  return { matched, text: text.slice(-20000) };
}
