import { render, screen, fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import SchedulerRunningBanner from './SchedulerRunningBanner';

describe('SchedulerRunningBanner', () => {
  it('calls onCancel when Cancel run is clicked', () => {
    const onCancel = vi.fn();
    render(<SchedulerRunningBanner cancelLoading={false} onCancel={onCancel} />);

    fireEvent.click(screen.getByRole('button', { name: /cancel run/i }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('disables the button while cancel is in flight (label is replaced by spinner)', () => {
    const onCancel = vi.fn();
    render(<SchedulerRunningBanner cancelLoading onCancel={onCancel} />);

    const btn = screen.getByRole('button');
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute('aria-busy', 'true');
  });
});
