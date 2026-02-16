import { h } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { voiceState, lastTranscript, voiceError } from '../state/voiceState';

export function VoiceIndicator() {
  const state = voiceState.value;
  const transcript = lastTranscript.value;
  const error = voiceError.value;
  const [showTranscript, setShowTranscript] = useState(false);

  useEffect(() => {
    if (transcript) {
      setShowTranscript(true);
      const timer = setTimeout(() => {
        setShowTranscript(false);
      }, 2000);
      return () => clearTimeout(timer);
    }
  }, [transcript]);

  const getStateColor = (): string => {
    switch (state) {
      case 'listening':
        return 'text-green-400';
      case 'processing':
        return 'text-yellow-400';
      case 'error':
        return 'text-red-400';
      default:
        return 'text-gray-400';
    }
  };

  const getPulseClass = (): string => {
    return state === 'listening' ? 'animate-pulse' : '';
  };

  return h('div', { className: 'absolute top-0 right-0 flex flex-col items-end' },
    h('div', {
      className: `flex items-center gap-2 ${getStateColor()} ${getPulseClass()}`,
      title: error || state,
    },
      h('svg', {
        xmlns: 'http://www.w3.org/2000/svg',
        className: 'h-5 w-5',
        fill: 'none',
        viewBox: '0 0 24 24',
        stroke: 'currentColor',
        strokeWidth: 2,
      },
        h('path', {
          strokeLinecap: 'round',
          strokeLinejoin: 'round',
          d: 'M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z',
        })
      )
    ),
    showTranscript && transcript && h('div', {
      className: 'mt-1 text-xs text-white/60 max-w-32 truncate',
    }, transcript)
  );
}
