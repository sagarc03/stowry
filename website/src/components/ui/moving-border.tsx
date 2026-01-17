'use client';
import React from 'react';
import {
  motion,
  useAnimationFrame,
  useMotionTemplate,
  useMotionValue,
  useTransform,
} from 'motion/react';
import { useRef, useEffect } from 'react';
import { cn } from '@/lib/utils';

export const MovingBorder = ({
  children,
  duration = 3000,
  rx,
  ry,
  isAnimating = true,
  progressPercentage = 0.485,
  ...otherProps
}: {
  children: React.ReactNode;
  duration?: number;
  rx?: string;
  ry?: string;
  isAnimating?: boolean;
  progressPercentage?: number;
  [key: string]: any;
}) => {
  const pathRef = useRef<any>(null);
  const progress = useMotionValue<number>(progressPercentage);
  const lastTimeRef = useRef<number>(0);
  const lastProgressRef = useRef<number>(0);
  const isFirstFrameRef = useRef<boolean>(true);
  const isInitializedRef = useRef<boolean>(false);

  useAnimationFrame((time) => {
    if (!isAnimating) {
      lastProgressRef.current = progress.get();
      lastTimeRef.current = time;
      isFirstFrameRef.current = true;
      return;
    }

    const length = pathRef.current?.getTotalLength();
    if (length) {
      if (isFirstFrameRef.current) {
        lastTimeRef.current = time;
        isFirstFrameRef.current = false;
        return;
      }

      const pxPerMillisecond = length / duration;
      const timeDiff = time - lastTimeRef.current;
      const progressDiff = (timeDiff * pxPerMillisecond) % length;

      const newProgress = lastProgressRef.current + progressDiff;
      progress.set(newProgress % length);

      lastTimeRef.current = time;
      lastProgressRef.current = newProgress % length;
    }
  });

  const x = useTransform(
    progress,
    (val) => pathRef.current?.getPointAtLength(val).x,
  );
  const y = useTransform(
    progress,
    (val) => pathRef.current?.getPointAtLength(val).y,
  );

  const transform = useMotionTemplate`translateX(${x}px) translateY(${y}px) translateX(-50%) translateY(-50%)`;

  return (
    <>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        preserveAspectRatio="none"
        className="absolute h-full w-full"
        width="100%"
        height="100%"
        {...otherProps}
      >
        <rect
          fill="none"
          width="100%"
          height="100%"
          rx={rx}
          ry={ry}
          ref={pathRef}
        />
      </svg>
      <motion.div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          display: 'inline-block',
          transform,
        }}
      >
        {children}
      </motion.div>
    </>
  );
};
