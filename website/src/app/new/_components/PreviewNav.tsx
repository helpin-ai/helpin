'use client';

import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent as ReactPointerEvent } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { ArrowRight, ArrowUpRight, BookOpen, Bot, Braces, Building2, ChevronDown, Kanban, Menu, MessagesSquare, Plug, Server, Terminal, Users, Video, Webhook, X } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';
import { GITHUB_URL, SIGNUP_URL, GithubIcon } from './ui';

const PRODUCTS = [
  { label: 'Customer support', description: 'Answers, handoffs, and a shared inbox.', href: '/new/products/customer-support', icon: MessagesSquare },
  { label: 'Meetings', description: 'Conversations become next steps.', href: '/new/product#meetings', icon: Video },
  { label: 'Projects', description: 'Customer requests connected to work.', href: '/new/product#projects', icon: Kanban },
  { label: 'CRM', description: 'The history behind every account.', href: '/new/product#crm', icon: Building2 },
  { label: 'Knowledge', description: 'Answers for customers and agents.', href: '/new/product#knowledge', icon: BookOpen },
  { label: 'Customer records', description: 'One customer. The full picture.', href: '/new#record', icon: Users },
];
const DEVELOPERS = [
  { label: 'APIs & SDKs', description: 'Connect Helpin to your product.', href: '/new#developers', icon: Braces },
  { label: 'MCP', description: 'Give AI tools workspace context.', href: `${GITHUB_URL}/blob/develop/docs/public-mcp-server.md`, icon: Plug },
  { label: 'Webhooks', description: 'Build around workspace events.', href: `${GITHUB_URL}/blob/develop/docs/README.md`, icon: Webhook },
  { label: 'Helpin CLI', description: 'Set up and manage your instance.', href: `${GITHUB_URL}/blob/develop/community/README.md`, icon: Terminal },
];
type MenuName = 'product' | 'developers';

function ResourceLink({ item }: { item: typeof PRODUCTS[number] }) {
  const Icon = item.icon;
  const contents = <><span className="nav-resource-icon"><Icon size={19} strokeWidth={1.6} /></span><span><b>{item.label}</b><small>{item.description}</small></span>{item.href.startsWith('https:') ? <ArrowUpRight className="nav-external" size={13} /> : null}</>;
  return item.href.startsWith('https:')
    ? <a className="nav-resource" href={item.href} target="_blank" rel="noopener noreferrer">{contents}</a>
    : <Link className="nav-resource" href={item.href}>{contents}</Link>;
}

export function PreviewNav() {
  const pathname = usePathname();
  const [hidden, setHidden] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const [open, setOpen] = useState<MenuName | null>(null);
  const [mobileOpen, setMobileOpen] = useState(false);
  const nav = useRef<HTMLElement>(null);
  const mobileToggle = useRef<HTMLButtonElement>(null);
  const productToggle = useRef<HTMLButtonElement>(null);
  const developerToggle = useRef<HTMLButtonElement>(null);
  const hoverClose = useRef<ReturnType<typeof setTimeout> | null>(null);
  const openedByHover = useRef(false);
  const cancelHoverClose = () => { if (hoverClose.current) clearTimeout(hoverClose.current); };
  const expanded = open !== null || mobileOpen;
  const close = () => { cancelHoverClose(); openedByHover.current = false; setOpen(null); setMobileOpen(false); };
  const enterProduct = (event: ReactPointerEvent<HTMLElement>) => {
    if (event.pointerType !== 'mouse') return;
    cancelHoverClose();
    if (open !== 'product') { openedByHover.current = true; setOpen('product'); }
  };
  const leaveProduct = (event: ReactPointerEvent<HTMLElement>) => {
    if (event.pointerType !== 'mouse') return;
    cancelHoverClose();
    // Let the pointer cross the small gap between the trigger and the panel.
    hoverClose.current = setTimeout(() => {
      if (productToggle.current?.matches(':focus-visible') || nav.current?.querySelector('#preview-product-menu :focus-visible')) return;
      openedByHover.current = false;
      setOpen(current => current === 'product' ? null : current);
    }, 200);
  };
  useEffect(() => () => { if (hoverClose.current) clearTimeout(hoverClose.current); }, []);

  useEffect(() => { setOpen(null); setMobileOpen(false); }, [pathname]);

  useEffect(() => {
    const onOutside = (event: PointerEvent) => {
      if (event.target instanceof Node && !nav.current?.contains(event.target)) {
        setOpen(null); setMobileOpen(false);
      }
    };
    const breakpoint = window.matchMedia('(max-width: 1000px)');
    const onResize = () => { setOpen(null); setMobileOpen(false); };
    document.addEventListener('pointerdown', onOutside);
    breakpoint.addEventListener('change', onResize);
    return () => { document.removeEventListener('pointerdown', onOutside); breakpoint.removeEventListener('change', onResize); };
  }, []);

  useEffect(() => {
    if (!mobileOpen) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => { document.body.style.overflow = previousOverflow; };
  }, [mobileOpen]);

  useEffect(() => {
    let previousY = Math.max(0, window.scrollY);
    let travel = 0;
    let frame = 0;
    const update = () => {
      frame = 0;
      const y = Math.max(0, window.scrollY);
      const delta = y - previousY;
      previousY = y;
      setScrolled(y > 12);
      if (y <= 60 || expanded || nav.current?.querySelector(':focus-visible')) {
        travel = 0; setHidden(false); return;
      }
      if (!delta) return;
      if (Math.sign(delta) !== Math.sign(travel)) travel = 0;
      travel += delta;
      if (travel > 20) setHidden(true);
      else if (travel < -10) setHidden(false);
    };
    update();
    const onScroll = () => { if (!frame) frame = window.requestAnimationFrame(update); };
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => { window.removeEventListener('scroll', onScroll); window.cancelAnimationFrame(frame); };
  }, [expanded]);

  const keyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === 'Escape' && expanded) {
      event.preventDefault();
      const trigger = mobileOpen ? mobileToggle : open === 'product' ? productToggle : developerToggle;
      close(); trigger.current?.focus();
    }
    if (event.key === 'Tab' && mobileOpen && nav.current) {
      const focusable = [...nav.current.querySelectorAll<HTMLElement>('a[href], button')].filter(el => el.getClientRects().length > 0);
      const first = focusable[0], last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    }
  };
  const openWithKeyboard = (event: KeyboardEvent<HTMLButtonElement>, name: MenuName) => {
    if (event.key !== 'ArrowDown') return;
    event.preventDefault(); cancelHoverClose(); openedByHover.current = false; setOpen(name);
    window.requestAnimationFrame(() => nav.current?.querySelector<HTMLElement>(`#preview-${name}-menu a`)?.focus());
  };

  return (
    <>
      {expanded ? <div className="nav-backdrop" aria-hidden="true" onClick={close} /> : null}
      <nav ref={nav} className="pnav" aria-label="Main navigation" data-hidden={hidden && !expanded} data-scrolled={scrolled} data-expanded={expanded}
        onKeyDown={keyDown}
        onBlurCapture={event => { if (event.relatedTarget instanceof Node && !event.currentTarget.contains(event.relatedTarget)) close(); }}
        onFocusCapture={event => { if (event.target.matches(':focus-visible')) setHidden(false); }}>
        <div className="wrap nav-bar">
          <Link href="/new" className="logo" aria-label="Helpin homepage" onClick={close}><HelpinBrand /></Link>
          <div className="navlinks">
            <button ref={productToggle} className="nav-trigger" aria-expanded={open === 'product'} aria-controls="preview-product-menu" onPointerEnter={enterProduct} onPointerLeave={leaveProduct} onClick={event => { cancelHoverClose(); setOpen(openedByHover.current && event.detail > 0 ? 'product' : open === 'product' ? null : 'product'); openedByHover.current = false; }} onKeyDown={event => openWithKeyboard(event, 'product')}>Product<ChevronDown size={13} /></button>
            <Link className="nav-direct" href="/new/products/ai-agents" onClick={close}>AI Agents</Link>
            <button ref={developerToggle} className="nav-trigger" aria-expanded={open === 'developers'} aria-controls="preview-developers-menu" onClick={() => setOpen(open === 'developers' ? null : 'developers')} onKeyDown={event => openWithKeyboard(event, 'developers')}>Developers<ChevronDown size={13} /></button>
            <Link className="nav-direct" href="/new#open-source" onClick={close}>Open source</Link>
          </div>
          <div className="navright">
            <a className="gh" href={GITHUB_URL} target="_blank" rel="noopener noreferrer" aria-label="Helpin on GitHub"><GithubIcon /><span>GitHub</span></a>
            <a className="nav-signin" href="https://app.helpin.ai">Sign in</a>
            <Link className="btn btn-primary" href={SIGNUP_URL} onClick={close}>Start free<ArrowRight size={15} /></Link>
            <button ref={mobileToggle} className="nav-mobile-toggle" type="button" aria-expanded={mobileOpen} aria-controls="preview-mobile-menu" aria-label={mobileOpen ? 'Close navigation' : 'Open navigation'} onClick={() => { setMobileOpen(!mobileOpen); setOpen(null); }}>{mobileOpen ? <X size={21} /> : <Menu size={21} />}</button>
          </div>
        </div>

        <div id="preview-product-menu" className="nav-panel nav-product-panel" onPointerEnter={enterProduct} onPointerLeave={leaveProduct} hidden={open !== 'product'} onClick={event => { if ((event.target as HTMLElement).closest('a')) close(); }}>
          <div className="nav-panel-main">
            <p className="nav-section-label">Your customer workspace</p>
            <div className="nav-resource-grid">{PRODUCTS.map(item => <ResourceLink item={item} key={item.label} />)}</div>
            <Link className="nav-panel-footer" href="/new/product">Explore the platform<ArrowRight size={14} /></Link>
          </div>
          <Link href="/new/products/ai-agents#agent-ask" className="nav-agent-feature">
            <span className="nav-feature-mark"><Bot size={25} strokeWidth={1.5} /></span>
            <span className="nav-section-label">Meet Ask Agent</span>
            <strong>Ask a question.<br />Hand off the work.</strong>
            <p>Find the context. Coordinate agents. Get the work moving.</p>
            <span className="nav-feature-cta">Explore Ask Agent<ArrowRight size={14} /></span>
          </Link>
        </div>

        <div id="preview-developers-menu" className="nav-panel nav-developer-panel" hidden={open !== 'developers'} onClick={event => { if ((event.target as HTMLElement).closest('a')) close(); }}>
          <div className="nav-panel-main">
            <p className="nav-section-label">Build with Helpin</p>
            <div className="nav-resource-grid">{DEVELOPERS.map(item => <ResourceLink item={item} key={item.label} />)}</div>
            <div className="nav-developer-footer">
              <a href={`${GITHUB_URL}/blob/develop/docs/README.md`} target="_blank" rel="noopener noreferrer"><BookOpen size={15} />Read the docs<ArrowUpRight size={13} /></a>
              <Link href="/new#open-source"><Server size={15} />Self-host Helpin<ArrowRight size={13} /></Link>
            </div>
          </div>
        </div>

        <div id="preview-mobile-menu" className="nav-mobile-panel" hidden={!mobileOpen} onClick={event => { if ((event.target as HTMLElement).closest('a')) close(); }}>
          <Link className="nav-mobile-agent" href="/new/products/ai-agents"><Bot size={21} /><span><b>AI Agents</b><small>Meet the agents behind the work.</small></span><ArrowRight size={17} /></Link>
          <p className="nav-section-label">Product</p>
          <div className="nav-mobile-products">{PRODUCTS.map(item => <ResourceLink item={item} key={item.label} />)}</div>
          <Link className="nav-mobile-overview" href="/new/product">Explore the platform<ArrowRight size={14} /></Link>
          <div className="nav-mobile-resources">
            <Link href="/new#developers"><Braces size={17} />Developers<ArrowRight size={13} /></Link>
            <Link href="/new#open-source"><Server size={17} />Open source & self-hosting<ArrowRight size={13} /></Link>
            <a href={`${GITHUB_URL}/blob/develop/docs/README.md`} target="_blank" rel="noopener noreferrer"><BookOpen size={17} />Documentation<ArrowUpRight size={13} /></a>
          </div>
          <div className="nav-mobile-bottom"><a href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon size={17} />View on GitHub</a><a href="https://app.helpin.ai">Sign in<ArrowRight size={14} /></a></div>
        </div>
      </nav>
    </>
  );
}
