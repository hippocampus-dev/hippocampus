import { h } from 'preact';
import { useRef } from 'preact/hooks';
import { Timer } from '../types/timer';
import { TimerCard } from './TimerCard';

interface TimerCardStackProps {
  timers: Timer[];
  viewIndex: number;
  isEditable: boolean;
  onViewIndexChange: (index: number) => void;
  onRemove?: (index: number) => void;
}

export function TimerCardStack({
  timers,
  viewIndex,
  isEditable,
  onViewIndexChange,
  onRemove,
}: TimerCardStackProps) {
  const touchStartX = useRef<number>(0);
  const touchStartY = useRef<number>(0);

  const handleTouchStart = (e: TouchEvent) => {
    touchStartX.current = e.touches[0].clientX;
    touchStartY.current = e.touches[0].clientY;
  };

  const handleTouchEnd = (e: TouchEvent) => {
    if (!isEditable) return;

    const touchEndX = e.changedTouches[0].clientX;
    const touchEndY = e.changedTouches[0].clientY;
    const deltaX = touchEndX - touchStartX.current;
    const deltaY = touchEndY - touchStartY.current;

    if (Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > 50) {
      if (deltaX < 0 && viewIndex < timers.length - 1) {
        onViewIndexChange(viewIndex + 1);
      } else if (deltaX > 0 && viewIndex > 0) {
        onViewIndexChange(viewIndex - 1);
      }
    }
  };

  const displayIndex = viewIndex;

  const getCardStyle = (position: number) => {
    if (position < 0) {
      const translateY = position * 16;
      const translateZ = position * 40;
      const scale = 1 + position * 0.05;
      const opacity = 1 + position * 0.2;
      return {
        transform: `translateY(${translateY}px) translateZ(${translateZ}px) scale(${scale})`,
        opacity: Math.max(0.3, opacity),
        zIndex: 30 + position,
        pointerEvents: 'none' as const,
      };
    }

    const translateY = -position * 16;
    const translateZ = -position * 40;
    const scale = 1 - position * 0.05;
    const opacity = 1 - position * 0.2;
    const zIndex = 30 - position;

    return {
      transform: `translateY(${translateY}px) translateZ(${translateZ}px) scale(${scale})`,
      opacity: Math.max(0.3, opacity),
      zIndex,
      pointerEvents: position === 0 ? 'auto' as const : 'none' as const,
    };
  };

  return h('div', {
    className: 'relative h-56 z-10 isolate',
    style: { perspective: '1000px' },
    onTouchStart: handleTouchStart,
    onTouchEnd: handleTouchEnd,
  },
    timers.map((timer, index) => {
      const position = index - displayIndex;
      if (position < -1 || position > 3) return null;
      const currentTimer = timers[displayIndex];
      if (currentTimer?.status === 'running' && position !== 0) return null;

      const style = getCardStyle(position);

      return h('div', {
        key: timer.id,
        className: 'absolute w-full transition-all duration-300 ease-out',
        style,
      },
        h(TimerCard, {
          timer,
          index,
          isRunning: timer.status === 'running' || timer.status === 'paused',
          onRemove,
          showRemove: isEditable && timers.length > 1 && position === 0,
        })
      );
    }),
    timers[displayIndex]?.status !== 'running' && timers.length > 1 && h('div', {
      className: 'absolute bottom-6 left-0 right-0 flex justify-center gap-2',
    },
      timers.map((_, index) =>
        h('div', {
          key: index,
          className: `w-2 h-2 rounded-full transition-all ${
            index === displayIndex
              ? 'bg-white scale-125'
              : 'bg-white/40'
          }`,
        })
      )
    )
  );
}
