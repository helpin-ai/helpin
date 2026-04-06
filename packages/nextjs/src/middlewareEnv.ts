import { NextRequest, NextResponse } from 'next/server';
import { ClientProperties } from '@helpin-ai/sdk-js';
import { serialize, CookieSerializeOptions } from 'cookie';

// Helper type guards for Next.js 13.2+ cookie API
function hasGetMethod(
  cookies: any,
): cookies is { get(name: string): { value: string } | undefined } {
  return cookies && typeof cookies.get === 'function';
}

interface ModernCookies {
  set(options: {
    name: string;
    value: string;
    maxAge?: number;
    domain?: string;
    httpOnly?: boolean;
  }): void;
}

function hasSetMethod(cookies: any): cookies is ModernCookies {
  return cookies && typeof cookies.set === 'function';
}

function middlewareEnv(
  req: NextRequest,
  res: NextResponse,
  opts: { disableCookies?: boolean } = {},
) {
  return {
    getAnonymousId({
      name,
      domain,
    }: {
      name: string;
      domain?: string;
    }): string {
      if (opts?.disableCookies) {
        return '';
      }

      // Handle cookies based on Next.js version
      let cookie: string | undefined;

      // Modern Next.js (13.2+, 14, 15)
      if (hasGetMethod(req.cookies)) {
        const cookieObj = req.cookies.get(name);
        cookie = cookieObj?.value;
      }
      // Legacy Next.js (pre-13.2)
      else if (req.cookies && typeof req.cookies === 'object') {
        cookie = (req.cookies as Record<string, string>)[name];
      }

      if (cookie) {
        return cookie;
      }

      const cookieOpts: CookieSerializeOptions = {
        maxAge: 31_622_400 * 10,
        httpOnly: false,
        path: '/',
      };
      if (domain) {
        cookieOpts.domain = domain;
      }

      let newId = Math.random().toString(36).substring(2, 12);

      // Handle cookie setting based on Next.js versions
      // NextResponse.cookies API was introduced in Next.js 13.2
      if (res.cookies && hasSetMethod(res.cookies)) {
        // Modern Next.js (13.2+, 14, 15)
        res.cookies.set({
          name,
          value: newId,
          maxAge: cookieOpts.maxAge,
          domain: cookieOpts.domain,
          httpOnly: cookieOpts.httpOnly,
        });
      } else {
        // Legacy Next.js (pre-13.2)
        res.headers.set('Set-Cookie', serialize(name, newId, cookieOpts));
      }

      return newId;
    },
    getSourceIp() {
      // Get IP from headers only as req.ip is not available in newer Next.js versions
      let ip =
        req.headers.get('x-forwarded-for') ||
        req.headers.get('x-real-ip') ||
        '';
      return ip && ip.split(',')[0].trim();
    },
    describeClient(): ClientProperties {
      const requestHost =
        req.headers.get('x-forwarded-host') ||
        req.headers.get('host') ||
        req.nextUrl.hostname;
      const proto =
        req.headers.get('x-forwarded-proto') ||
        req.nextUrl.protocol?.replace(':', '');
      let query = req.nextUrl.search;
      let path = req.nextUrl.pathname;
      return {
        doc_encoding: '',
        doc_host: requestHost,
        doc_path: req.url,
        doc_search: query,
        page_title: '',
        referer: req.headers.get('referrer'),
        screen_resolution: '',
        url: `${proto}://${requestHost}${path || ''}${query || ''}`,
        user_agent: req.headers.get('user-agent'),
        user_language:
          req.headers.get('accept-language') &&
          req.headers.get('accept-language').split(',')[0],
        vp_size: '',
      };
    },
  };
}

export default middlewareEnv;
