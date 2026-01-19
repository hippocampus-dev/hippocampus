import { h } from 'preact';
import { TimerView } from './TimerView';
import { SavedGroups } from './SavedGroups';

export function App() {
  const handleTimerComplete = async () => {
    try {
      if ('vibrate' in navigator) {
        navigator.vibrate([200, 100, 200]);
      }
    } catch {
      // Vibration not supported
    }
  };

  return h('div', {
    className: 'h-screen flex flex-col p-6 text-white overflow-hidden',
  },
    h('header', { className: 'text-center mb-8' },
      h('h1', { className: 'text-2xl font-bold' }, 'Taurim Timer')
    ),
    h('main', { className: 'flex-1 flex flex-col gap-6 max-w-md mx-auto w-full min-h-0' },
      h('div', { className: 'flex-shrink-0 pt-8' },
        h(TimerView, { onTimerComplete: handleTimerComplete })
      ),
      h('div', { className: 'flex-1 min-h-0 flex flex-col' },
        h('hr', { className: 'border-white/20 flex-shrink-0 mb-6' }),
        h(SavedGroups, {})
      )
    )
  );
}
