import { h } from 'preact';
import { useEffect, useRef } from 'preact/hooks';
import {
  timerGroup,
  viewIndex,
  currentTimer,
  isRunning,
  isAlarmPlaying,
  addTimer,
  removeTimer,
  updateTimerDuration,
  setViewIndex,
  startTimer,
  pauseTimer,
  resetTimer,
  tick,
  advanceToNextTimer,
  startAlarm,
  stopAlarm,
} from '../state/timerState';
import { TimerControls } from './TimerControls';
import { TimerCardStack } from './TimerCardStack';
import { TimerInput } from './TimerInput';
import { startBackgroundTimer, stopBackgroundTimer, stopBackgroundAlarm, isAndroid, playAlarm, stopAlarmSound } from '../services/timerService';

interface TimerViewProps {
  onTimerComplete: () => void;
}

export function TimerView({ onTimerComplete }: TimerViewProps) {
  const intervalRef = useRef<number | null>(null);

  const handleStart = async () => {
    const timer = currentTimer.value;
    if (timer && isAndroid()) {
      try {
        await startBackgroundTimer(timer.remainingSeconds);
      } catch {
        // Continue with foreground timer
      }
    }
    startTimer();
  };

  const handlePause = async () => {
    if (isAndroid()) {
      try {
        await stopBackgroundTimer();
      } catch {
        // Continue
      }
    }
    pauseTimer();
  };

  const handleReset = async () => {
    if (!isAndroid()) stopAlarmSound();
    stopAlarm();
    if (isAndroid()) {
      try {
        await stopBackgroundTimer();
      } catch {
        // Continue
      }
    }
    resetTimer();
  };

  const handleStopAlarm = async () => {
    if (!isAndroid()) stopAlarmSound();
    if (isAndroid()) {
      try {
        await stopBackgroundAlarm();
      } catch {
        // Continue
      }
    }
    stopAlarm();
    advanceToNextTimer();
  };

  useEffect(() => {
    if (isRunning.value && !isAndroid()) {
      intervalRef.current = window.setInterval(() => {
        const completed = tick();
        if (completed) {
          startAlarm();
          playAlarm();
          onTimerComplete();
        }
      }, 1000);
    } else if (intervalRef.current !== null) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }

    return () => {
      if (intervalRef.current !== null) {
        clearInterval(intervalRef.current);
      }
      if (!isAndroid()) stopAlarmSound();
    };
  }, [isRunning.value, onTimerComplete]);

  const group = timerGroup.value;
  const running = isRunning.value;
  const isEditable = group.status === 'idle';

  const currentViewIndex = viewIndex.value;
  const currentTimerDuration = group.timers[currentViewIndex]?.durationSeconds ?? 60;

  return h('div', { className: 'flex flex-col h-[420px]' },
    h(TimerCardStack, {
      timers: group.timers,
      viewIndex: currentViewIndex,
      isEditable,
      onViewIndexChange: setViewIndex,
      onRemove: removeTimer,
    }),
    h('div', { className: 'flex-1 flex flex-col justify-end gap-4' },
      isEditable ? h(TimerInput, {
        value: currentTimerDuration,
        onChange: (seconds) => updateTimerDuration(currentViewIndex, seconds),
      }) : null,
      isEditable ? h('div', { className: 'flex justify-center' },
        h('button', {
          onClick: () => addTimer(60),
          className: `
            px-4 py-2 rounded-lg
            bg-white/10 hover:bg-white/20
            text-white text-sm font-medium
            border border-white/20
            transition-colors
          `,
        }, '+ Add Timer')
      ) : null,
      h(TimerControls, {
        isRunning: running,
        isCompleted: group.status === 'completed',
        isAlarmPlaying: isAlarmPlaying.value,
        onStart: handleStart,
        onPause: handlePause,
        onReset: handleReset,
        onStopAlarm: handleStopAlarm,
      })
    )
  );
}
