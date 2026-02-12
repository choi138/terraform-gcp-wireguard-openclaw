module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'type-enum': [2, 'always', ['feat', 'fix', 'chore', 'refactor']],
  },
  ignores: [
    (message) => message.startsWith('Merge '),
    (message) => message.startsWith('Revert '),
  ],
};
