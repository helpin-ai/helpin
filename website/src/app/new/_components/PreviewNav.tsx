'use client';

import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent as ReactPointerEvent } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { ArrowRight, ArrowUpRight, BookOpen, Braces, ChevronDown, Menu, Plug, Server, X } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';
import { GITHUB_URL, SIGNUP_URL, GithubIcon } from './ui';
import { PRODUCTS, ProductLink, ProductsMenu } from './ProductsMenu';

const DEVELOPERS = [
  { label: 'Docs', description: 'Setup guides and technical reference.', href: `${GITHUB_URL}/blob/develop/docs/README.md`, icon: BookOpen },
  { label: 'APIs & SDKs', description: 'Connect Helpin to your product.', href: '/new/developers', icon: Braces },
  { label: 'MCP', description: 'Give AI tools access to Helpin.', href: '/new/developers#mcp', icon: Plug },
  { label: 'Self-hosting', description: 'Run Helpin on your infrastructure.', href: '/new/self-hosting', icon: Server },
];
type MenuName = 'product' | 'developers';

function ResourceLink({ item }: { item: typeof DEVELOPERS[number] }) {
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
            <button ref={productToggle} className="nav-trigger" aria-expanded={open === 'product'} aria-controls="preview-product-menu" onPointerEnter={enterProduct} onPointerLeave={leaveProduct} onClick={event => { cancelHoverClose(); setOpen(openedByHover.current && event.detail > 0 ? 'product' : open === 'product' ? null : 'product'); openedByHover.current = false; }} onKeyDown={event => openWithKeyboard(event, 'product')}>Products<ChevronDown size={13} /></button>

            <button ref={developerToggle} className="nav-trigger" aria-expanded={open === 'developers'} aria-controls="preview-developers-menu" onClick={() => setOpen(open === 'developers' ? null : 'developers')} onKeyDown={event => openWithKeyboard(event, 'developers')}>Developers<ChevronDown size={13} /></button>
            <Link className="nav-direct" href="/pricing" onClick={close}>Pricing</Link>
          </div>
          <div className="navright">
            <a className="gh" href={GITHUB_URL} target="_blank" rel="noopener noreferrer" aria-label="Helpin on GitHub"><GithubIcon /><span>GitHub</span></a>
            <a className="nav-signin" href="https://app.helpin.ai">Sign in</a>
            <Link className="btn btn-primary" href={SIGNUP_URL} onClick={close}>Start free<ArrowRight size={15} /></Link>
            <button ref={mobileToggle} className="nav-mobile-toggle" type="button" aria-expanded={mobileOpen} aria-controls="preview-mobile-menu" aria-label={mobileOpen ? 'Close navigation' : 'Open navigation'} onClick={() => { setMobileOpen(!mobileOpen); setOpen(null); }}>{mobileOpen ? <X size={21} /> : <Menu size={21} />}</button>
          </div>
        </div>

        <div id="preview-product-menu" className="nav-panel nav-product-panel" onPointerEnter={enterProduct} onPointerLeave={leaveProduct} hidden={open !== 'product'} onClick={event => { if ((event.target as HTMLElement).closest('a')) close(); }}>
          <ProductsMenu />
        </div>

        <div id="preview-developers-menu" className="nav-panel nav-developer-panel" hidden={open !== 'developers'} onClick={event => { if ((event.target as HTMLElement).closest('a')) close(); }}>
          <div className="nav-panel-main">
            <p className="nav-section-label">Build with Helpin</p>
            <div className="nav-resource-grid">{DEVELOPERS.map(item => <ResourceLink item={item} key={item.label} />)}</div>
            <div className="nav-developer-footer">
              <a href={`${GITHUB_URL}/blob/develop/docs/README.md`} target="_blank" rel="noopener noreferrer"><BookOpen size={15} />Read the docs<ArrowUpRight size={13} /></a>
              <Link href="/new/self-hosting"><Server size={15} />Self-host Helpin<ArrowRight size={13} /></Link>
            </div>
          </div>
        </div>

        <div id="preview-mobile-menu" className="nav-mobile-panel" hidden={!mobileOpen} onClick={event => { if ((event.target as HTMLElement).closest('a')) close(); }}>

          <p className="nav-section-label">Products</p>
          <div className="nav-mobile-products">{PRODUCTS.map(item => <ProductLink item={item} key={item.label} />)}</div>
          <Link className="nav-mobile-overview" href="/new/product">Explore the platform<ArrowRight size={14} /></Link>
          <div className="nav-mobile-resources"><Link href="/pricing">Pricing<ArrowRight size={13} /></Link>
            <Link href="/new/developers"><Braces size={17} />Developers<ArrowRight size={13} /></Link>
            <Link href="/new/self-hosting"><Server size={17} />Open source & self-hosting<ArrowRight size={13} /></Link>
            <a href={`${GITHUB_URL}/blob/develop/docs/README.md`} target="_blank" rel="noopener noreferrer"><BookOpen size={17} />Documentation<ArrowUpRight size={13} /></a>
          </div>
          <div className="nav-mobile-bottom"><a href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon size={17} />View on GitHub</a><a href="https://app.helpin.ai">Sign in<ArrowRight size={14} /></a></div>
        </div>
      </nav>
    </>
  );
}
