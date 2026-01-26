import { h } from 'preact';
import { useState } from 'preact/hooks';
import {
  timerGroup,
  savedGroups,
  saveCurrentGroup,
  loadGroup,
  deleteSavedGroup,
} from '../state/timerState';
import { SavedGroup, formatTime } from '../types/timer';

export function SavedGroups() {
  const [showSaveModal, setShowSaveModal] = useState(false);
  const [groupName, setGroupName] = useState('');

  const groups = savedGroups.value;
  const canSave = timerGroup.value.timers.length > 0;

  const handleSave = () => {
    if (groupName.trim()) {
      saveCurrentGroup(groupName.trim());
      setGroupName('');
      setShowSaveModal(false);
    }
  };

  return h('div', { className: 'flex flex-col gap-4 flex-1 min-h-0' },
    h('div', { className: 'flex items-center justify-between flex-shrink-0' },
      h('h2', { className: 'text-lg font-semibold text-white/80' }, 'Saved Groups'),
      canSave && h('button', {
        onClick: () => setShowSaveModal(true),
        className: `
          px-3 py-1 rounded-lg text-sm
          bg-white/10 hover:bg-white/20
          text-white border border-white/20
          transition-colors
        `,
      }, 'Save Current')
    ),
    groups.length === 0
      ? h('p', { className: 'text-white/50 text-sm text-center py-4' },
          'No saved groups yet')
      : h('div', {
          className: 'flex flex-col gap-2 flex-1 overflow-y-auto min-h-0 overscroll-contain',
          style: { WebkitOverflowScrolling: 'touch' },
        },
          groups.map((group: SavedGroup) =>
            h('div', {
              key: group.id,
              className: `
                flex items-center justify-between p-3 rounded-lg
                bg-white/10 hover:bg-white/15
                transition-colors cursor-pointer
              `,
              onClick: () => loadGroup(group),
            },
              h('div', { className: 'flex flex-col' },
                h('span', { className: 'text-white font-medium' }, group.name),
                h('span', { className: 'text-white/50 text-xs' },
                  group.durations.map((d: number) => formatTime(d)).join(' \u2192 ')
                )
              ),
              h('button', {
                onClick: (event: Event) => {
                  event.stopPropagation();
                  deleteSavedGroup(group.id);
                },
                className: `
                  w-6 h-6 rounded-full
                  bg-red-500/30 hover:bg-red-500/50
                  text-white text-sm
                  transition-colors
                `,
              }, '\u00d7')
            )
          )
        ),
    showSaveModal && h('div', {
      className: `
        fixed inset-0 bg-black/50 backdrop-blur-sm
        flex items-center justify-center z-50
      `,
      onClick: () => setShowSaveModal(false),
    },
      h('div', {
        className: `
          bg-white/20 backdrop-blur-lg rounded-2xl p-6
          border border-white/30 shadow-2xl
          min-w-72
        `,
        onClick: (event: Event) => event.stopPropagation(),
      },
        h('h3', { className: 'text-white text-lg font-semibold mb-4' },
          'Save Timer Group'),
        h('input', {
          type: 'text',
          value: groupName,
          onInput: (event: Event) => setGroupName((event.target as HTMLInputElement).value),
          placeholder: 'Group name',
          className: `
            w-full px-4 py-2 rounded-lg mb-4
            bg-white/10 border border-white/30
            text-white placeholder-white/50
            focus:outline-none focus:border-white/50
          `,
          onKeyDown: (event: KeyboardEvent) => {
            if (event.key === 'Enter') handleSave();
          },
        }),
        h('div', { className: 'flex gap-3 justify-end' },
          h('button', {
            onClick: () => setShowSaveModal(false),
            className: `
              px-4 py-2 rounded-lg
              bg-white/10 hover:bg-white/20
              text-white transition-colors
            `,
          }, 'Cancel'),
          h('button', {
            onClick: handleSave,
            disabled: !groupName.trim(),
            className: `
              px-4 py-2 rounded-lg
              bg-white text-primary font-medium
              disabled:opacity-50
              transition-colors
            `,
          }, 'Save')
        )
      )
    )
  );
}
