import { cn } from '@/lib/utils';

interface LogoProps {
  iconClassName?: string;
  wordmarkClassName?: string;
  className?: string;
  href?: string;
}

export default function Logo({
  iconClassName,
  wordmarkClassName,
  className,
  href = '/',
}: LogoProps) {
  return (
    <a href={href} className={cn('flex items-center gap-2.5', className)}>
      <img
        src="/layout/logo-icon.svg"
        alt="Scalar Logo"
        width={22}
        height={24}
        className={cn('object-contain', iconClassName)}
      />
      <img
        src="/layout/logo-wordmark.svg"
        alt="Scalar"
        width={51.353}
        height={14.009}
        className={cn('object-contain', wordmarkClassName)}
      />
    </a>
  );
}
