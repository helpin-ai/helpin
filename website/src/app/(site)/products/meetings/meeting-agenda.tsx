'use client';

import { useId, useState } from 'react';
import { CalendarDays, Check, ChevronDown, Pause, Play, Repeat2 } from 'lucide-react';
import { useMeetingPlayback } from './use-meeting-playback';
import './meeting-agenda.css';

type Choices = { customer: boolean; internal: boolean; series: boolean; first: boolean; second: boolean };

function CaptureSwitch({ label, checked, onChange }: { label: string; checked: boolean; onChange: () => void }) {
  return <button type="button" role="switch" aria-label={label} aria-checked={checked} className="ma-switch" onClick={onChange}><span /></button>;
}

// A local, interactive representation of Meetings + UpcomingCalendarMeetings.
// Series defaults and occurrence overrides are separate, as in the platform.
export function MeetingAgenda() {
  const { container, active, phase, paused, setPaused } = useMeetingPlayback();
  const [manual, setManual] = useState<Choices | null>(null);
  const [overrides, setOverrides] = useState<{ first?: boolean; second?: boolean } | null>(null);
  const [expanded, setExpanded] = useState(true);
  const [announcement, setAnnouncement] = useState('');
  const datesId = useId();
  const choices = manual ?? { customer: phase >= 1, internal: false, series: phase >= 2, first: phase >= 2, second: phase === 2 };
  const update = (key: keyof Choices, label: string) => {
    const enabled = !choices[key];
    setPaused(true);
    // Preserve date-specific overrides when the recurring-series default changes.
    const next = { ...choices, [key]: enabled };
    const dates = overrides ?? (phase === 3 ? { second: false } : {});
    if (key === 'series') {
      next.first = dates.first ?? enabled;
      next.second = dates.second ?? enabled;
    }
    setOverrides(key === 'first' || key === 'second' ? { ...dates, [key]: enabled } : dates);
    setManual(next);
    setAnnouncement(`${label}: auto-join ${enabled ? 'on' : 'off'} in this preview.`);
  };
  return <div className="meeting-agenda" ref={container} data-playing={active} data-phase={phase} role="region" aria-label="OrbitDesk upcoming meetings preview">
    <div className="ma-toolbar"><span><b className="mt-mark">O</b>OrbitDesk<span>/</span>Meetings</span><button type="button" className="ma-playback" aria-label={`${paused ? 'Play' : 'Pause'} meeting selection animation`} aria-pressed={paused} onClick={() => { setManual(null); setOverrides(null); setExpanded(true); setPaused(!paused); }}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div className="ma-body">
      <header className="ma-heading"><div><h3>Meetings</h3><p>Choose where Helpin joins you.</p></div><span className="ma-connected"><CalendarDays size={13} /><span>Google Calendar connected</span><Check size={12} /></span></header>
      <div className="ma-section-title"><h4>Upcoming <span>4</span></h4><span>Today–25</span></div>
      <div className="ma-row" data-selected={choices.customer}>
        <div className="ma-row-copy"><div className="ma-meta"><img src="/new/meetings/google_meet.svg" width={13} height={13} alt="" />Google Meet<span>Today · 10:00 AM</span></div><strong>Northstar Labs · SSO rollout review</strong><p>2 external attendees · Ready for automatic joining</p></div>
        <div className="ma-control"><span>Auto-join</span><CaptureSwitch label="Automatically join Northstar Labs SSO rollout review" checked={choices.customer} onChange={() => update('customer', 'Northstar Labs rollout review')} /></div>
      </div>
      <div className="ma-row" data-selected={choices.internal}>
        <div className="ma-row-copy"><div className="ma-meta"><img src="/new/meetings/zoom.svg" width={13} height={13} alt="" />Zoom<span>Today · 11:30 AM</span></div><strong>Product team check-in</strong><p>No external attendees</p></div>
        <div className="ma-control"><span>Auto-join</span><CaptureSwitch label="Automatically join Product team check-in" checked={choices.internal} onChange={() => update('internal', 'Product team check-in')} /></div>
      </div>
      <div className="ma-row ma-series" data-selected={choices.series}>
        <div className="ma-row-copy"><div className="ma-meta"><img src="/new/meetings/google_meet.svg" width={13} height={13} alt="" />Google Meet<span><Repeat2 size={11} />Recurring</span></div><strong>Northstar Labs · Weekly rollout sync</strong><p>2 external attendees · 2 upcoming occurrences</p><button type="button" className="ma-dates-toggle" aria-expanded={expanded} aria-controls={datesId} onClick={() => { setManual(choices); setOverrides(overrides ?? (phase === 3 ? { second: false } : {})); setPaused(true); setExpanded(!expanded); }}>{expanded ? 'Hide dates' : 'Show dates'}<ChevronDown size={12} /></button></div>
        <div className="ma-control"><span>Auto-join series</span><CaptureSwitch label="Automatically join recurring series Northstar Labs Weekly rollout sync" checked={choices.series} onChange={() => update('series', 'Weekly rollout series')} /></div>
      </div>
      <div id={datesId} className="ma-dates" hidden={!expanded}>
        <p>Keep the series setting or change a single date.</p>
        {(['first', 'second'] as const).map((key, index) => <div className="ma-occurrence" key={key} data-selected={choices[key]}><div><span>{index === 0 ? 'This Friday' : 'Next Friday'} · 2:00 PM</span><strong>Helpin will {choices[key] ? 'join' : 'skip'} this occurrence</strong></div><CaptureSwitch label={`Automatically join weekly rollout sync ${index === 0 ? 'this Friday' : 'next Friday'}`} checked={choices[key]} onChange={() => update(key, `${index === 0 ? 'This Friday' : 'Next Friday'} occurrence`)} /></div>)}
      </div>
    </div>
    <div className="ma-summary"><span className="ma-helpin"><img src="/brand/helpin-icon-white.svg" width={13} height={13} alt="" /></span><span>Choose a call, a whole series, or a single date.<small>The host may need to admit the notetaker.</small></span></div>
    <span className="mw-sr-only" role="status">{announcement}</span>
  </div>;
}
