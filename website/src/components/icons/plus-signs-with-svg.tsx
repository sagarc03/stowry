'use client';

import { useEffect, useRef, useState } from 'react';

import { cn } from '@/lib/utils';

interface PlusSignsProps {
  className?: string;
  plusSize?: number;
  spacing?: number;
  opacity?: number;
}

export const PlusSigns = ({
  className,
  plusSize = 6,
  spacing = 24,
  opacity = 0.15,
}: PlusSignsProps) => {
  const strokeWidth = 3;
  const svgRef = useRef<SVGSVGElement>(null);

  // State to hold plus signs data
  const [plusSigns, setPlusSigns] = useState<
    Array<{
      id: string;
      x: number;
      y: number;
      opacity: number;
    }>
  >([]);

  // Generate plus signs based on container dimensions
  useEffect(() => {
    if (!svgRef.current) return;

    const rect = svgRef.current.getBoundingClientRect();
    const width = rect.width || svgRef.current.clientWidth;
    const height = rect.height || svgRef.current.clientHeight;

    // If dimensions are 0, try again on next frame
    if (width === 0 || height === 0) {
      const timeoutId = setTimeout(() => {
        if (svgRef.current) {
          const newRect = svgRef.current.getBoundingClientRect();
          const newWidth = newRect.width || svgRef.current.clientWidth;
          const newHeight = newRect.height || svgRef.current.clientHeight;

          if (newWidth > 0 && newHeight > 0) {
            generateSigns(newWidth, newHeight);
          }
        }
      }, 100);

      return () => clearTimeout(timeoutId);
    }

    generateSigns(width, height);

    function generateSigns(w: number, h: number) {
      const signs = [];
      const cols = Math.ceil(w / spacing);
      const rows = Math.ceil(h / spacing);

      for (let row = 0; row < rows; row++) {
        for (let col = 0; col < cols; col++) {
          const x = col * spacing + spacing / 2;
          const y = row * spacing + spacing / 2;
          const randomOpacity = Math.random() * opacity;

          signs.push({
            id: `plus-${row}-${col}`,
            x,
            y,
            opacity: randomOpacity,
          });
        }
      }

      setPlusSigns(signs);
    }
  }, [spacing, opacity]);

  return (
    <svg
      ref={svgRef}
      className={cn('pointer-events-none h-full w-full', className)}
    >
      {plusSigns.map((sign) => (
        <g
          key={sign.id}
          transform={`translate(${sign.x}, ${sign.y})`}
          opacity={sign.opacity}
          className="fill-muted-foreground"
        >
          {/* Horizontal bar */}
          <rect
            x={-plusSize}
            y={-strokeWidth / 2}
            width={plusSize * 2}
            height={strokeWidth}
          />
          {/* Vertical bar */}
          <rect
            x={-strokeWidth / 2}
            y={-plusSize}
            width={strokeWidth}
            height={plusSize * 2}
          />
        </g>
      ))}
    </svg>
  );
};
