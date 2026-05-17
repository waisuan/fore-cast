-- Adds the alt-course days column to booking_presets. When non-empty, the
-- scheduler will fan out a second parallel booking attempt on the OPPOSITE
-- course (BRC <-> PLC) on the listed weekdays. NULL or empty = feature off.
-- Storage is a comma-separated upper-case 3-letter weekday list, e.g.
-- "MON,WED,SAT". Order is normalized at write time.
ALTER TABLE booking_presets
    ADD COLUMN IF NOT EXISTS alt_course_days TEXT;
