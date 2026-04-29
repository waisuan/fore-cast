'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { DayPicker } from 'react-day-picker';
import { MY_TIMEZONE } from '@/utils/date';
import 'react-day-picker/style.css';

interface DatePickerProps {
  id?: string;
  value: string;
  onChange: (isoDate: string) => void;
  min?: string;
  placeholder?: string;
  'aria-label'?: string;
}

function parseYmd(ymd: string | undefined): Date | undefined {
  if (!ymd) return undefined;
  const [y, m, d] = ymd.split('-').map(Number);
  if (!y || !m || !d) return undefined;
  return new Date(y, m - 1, d);
}

function toYmd(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function formatDisplay(ymd: string): string {
  const d = parseYmd(ymd);
  if (!d) return '';
  return d.toLocaleDateString('en-MY', {
    timeZone: MY_TIMEZONE,
    weekday: 'short',
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

/** Convert YYYY-MM-DD to DD/MM/YYYY for editing */
function toEditable(ymd: string): string {
  const d = parseYmd(ymd);
  if (!d) return '';
  return `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}/${d.getFullYear()}`;
}

/** Validate y/m/d actually exist on the calendar (rejects Feb 31 etc.) */
function isValidDate(y: number, m: number, d: number): boolean {
  const date = new Date(y, m - 1, d);
  return date.getFullYear() === y && date.getMonth() === m - 1 && date.getDate() === d;
}

/** Parse DD/MM/YYYY, D/M/YYYY, or YYYY-MM-DD into YYYY-MM-DD */
function parseTyped(raw: string): string | null {
  const t = raw.trim();
  const iso = t.match(/^(\d{4})-(\d{1,2})-(\d{1,2})$/);
  if (iso) {
    const [, ys, ms, ds] = iso;
    const y = Number(ys), m = Number(ms), d = Number(ds);
    if (isValidDate(y, m, d)) {
      return `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
    }
  }
  const dmy = t.match(/^(\d{1,2})[/.](\d{1,2})[/.](\d{4})$/);
  if (dmy) {
    const [, ds, ms, ys] = dmy;
    const y = Number(ys), m = Number(ms), d = Number(ds);
    if (isValidDate(y, m, d)) {
      return `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
    }
  }
  return null;
}

export default function DatePicker({
  id,
  value,
  onChange,
  min,
  placeholder = 'DD/MM/YYYY',
  'aria-label': ariaLabel,
}: DatePickerProps) {
  const [open, setOpen] = useState(false);
  const [inputText, setInputText] = useState('');
  const [editing, setEditing] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const selected = parseYmd(value);
  const disabledBefore = parseYmd(min);
  const defaultMonth = selected ?? disabledBefore ?? new Date();

  const startMonth = disabledBefore ?? new Date(new Date().getFullYear(), 0, 1);
  const endMonth = new Date(new Date().getFullYear() + 1, 11, 31);

  const handleSelect = useCallback(
    (day: Date | undefined) => {
      if (day) {
        onChange(toYmd(day));
      }
      setOpen(false);
      setEditing(false);
    },
    [onChange],
  );

  const commitTypedDate = useCallback(() => {
    const parsed = parseTyped(inputText);
    if (parsed) {
      const d = parseYmd(parsed);
      const tooEarly = d && disabledBefore && d < disabledBefore;
      if (!tooEarly) onChange(parsed);
    }
    setEditing(false);
  }, [inputText, onChange, disabledBefore]);

  const handleInputFocus = () => {
    setEditing(true);
    setInputText(value ? toEditable(value) : '');
  };

  const handleInputBlur = () => {
    if (inputText.trim()) {
      commitTypedDate();
    } else {
      setEditing(false);
    }
  };

  const handleInputKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      commitTypedDate();
      inputRef.current?.blur();
    }
    if (e.key === 'Escape') {
      setEditing(false);
      inputRef.current?.blur();
    }
  };

  useEffect(() => {
    if (!open) return;
    function onClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function onEscape(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', onClickOutside);
    document.addEventListener('keydown', onEscape);
    return () => {
      document.removeEventListener('mousedown', onClickOutside);
      document.removeEventListener('keydown', onEscape);
    };
  }, [open]);

  const displayValue = editing
    ? inputText
    : value
      ? formatDisplay(value)
      : '';

  return (
    <div ref={containerRef} className="relative">
      <div className="flex min-h-12 w-full items-center rounded border border-gray-300 bg-white touch-manipulation dark:border-gray-600 dark:bg-gray-700">
        <input
          ref={inputRef}
          id={id}
          type="text"
          value={displayValue}
          placeholder={placeholder}
          aria-label={ariaLabel}
          onChange={(e) => setInputText(e.target.value)}
          onFocus={handleInputFocus}
          onBlur={handleInputBlur}
          onKeyDown={handleInputKeyDown}
          className="min-w-0 flex-1 bg-transparent px-3 py-2.5 text-base text-gray-900 outline-none placeholder:text-gray-400 dark:text-white dark:placeholder:text-gray-500"
          data-date={value}
          autoComplete="off"
        />
        <button
          type="button"
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => setOpen((o) => !o)}
          className="shrink-0 px-3 py-2.5 text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300"
          aria-label="Open calendar"
          tabIndex={-1}
        >
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="h-5 w-5">
            <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5" />
          </svg>
        </button>
      </div>

      {open && (
        <div className="date-picker-popover absolute left-0 right-0 z-50 mt-1 w-max max-w-[calc(100vw-2rem)] rounded-lg border border-gray-200 bg-white p-2 shadow-lg sm:left-0 sm:right-auto dark:border-gray-600 dark:bg-gray-800">
          <DayPicker
            mode="single"
            captionLayout="dropdown"
            selected={selected}
            onSelect={handleSelect}
            defaultMonth={defaultMonth}
            startMonth={startMonth}
            endMonth={endMonth}
            disabled={disabledBefore ? { before: disabledBefore } : undefined}
            required
          />
        </div>
      )}
    </div>
  );
}
