export class CookieManager {
  private cookieDomain: string;

  constructor(private domain?: string) {
    this.cookieDomain = this.getCookieDomain();
  }

  set(
    name: string,
    value: string,
    expirationDays: number = 365,
    secure: boolean = true,
    httpOnly: boolean = false,
  ): void {
    const date = new Date();
    date.setTime(date.getTime() + expirationDays * 24 * 60 * 60 * 1000);
    const expires = `expires=${date.toUTCString()}`;
    const secureFlag = secure ? '; Secure' : '';
    const httpOnlyFlag = httpOnly ? '; HttpOnly' : '';
    document.cookie = `${name}=${value};${expires};path=/;domain=${this.cookieDomain}${secureFlag}${httpOnlyFlag}`;
  }

  get(name: string): string | null {
    const nameEQ = name + '=';
    const ca = document.cookie.split(';');
    for (let i = 0; i < ca.length; i++) {
      let c = ca[i].trim();
      if (c.indexOf(nameEQ) === 0) {
        return decodeURIComponent(c.substring(nameEQ.length));
      }
    }
    return null;
  }

  delete(name: string, path: string = '/'): void {
    document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=${path};domain=${this.cookieDomain}`;
  }

  private getCookieDomain(): string {
    if (typeof window === 'undefined' || this.domain) {
      return this.domain || '';
    }
    return this.extractRoot(window.location.hostname);
  }

  private extractRoot(url: string): string {
    // Check if it's an IP address
    if (this.isIpAddress(url)) {
      return url;
    }

    // Handle localhost
    if (url === 'localhost') {
      return url;
    }

    let rootDomain = this.extractTopLevelDomain(url);
    if (!rootDomain) {
      rootDomain = this.extractRootDomain(url);
    }

    return '.' + rootDomain;
  }

  private isIpAddress(url: string): boolean {
    const parts = url.split('.');
    return parts.length === 4 && parts.every((part) => !isNaN(Number(part)));
  }

  private extractHostname(url: string): string {
    let hostname: string;
    if (url.indexOf('//') > -1) {
      hostname = url.split('/')[2];
    } else {
      hostname = url.split('/')[0];
    }
    hostname = hostname.split(':')[0];
    hostname = hostname.split('?')[0];
    return hostname;
  }

  private extractRootDomain(url: string): string {
    let domain = this.extractHostname(url);
    const splitArr = domain.split('.');
    const arrLen = splitArr.length;

    if (arrLen > 2) {
      if (splitArr[arrLen - 1].length == 2) {
        // likely a ccTLD
        domain = splitArr[arrLen - 2] + '.' + splitArr[arrLen - 1];
        if (splitArr[arrLen - 2].length == 2) {
          domain = splitArr[arrLen - 3] + '.' + domain;
        }
      } else {
        // likely a gTLD
        domain = splitArr[arrLen - 2] + '.' + splitArr[arrLen - 1];
      }
    }
    return domain;
  }

  private extractTopLevelDomain(url: string): string {
    const DOMAIN_MATCH_REGEX = /[a-z0-9][a-z0-9-]+\.[a-z.]{2,6}$/i;
    const matches = url.match(DOMAIN_MATCH_REGEX);
    return matches ? matches[0] : '';
  }
}
