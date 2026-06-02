import { describe, expect, it } from 'vitest';

import { cleanForwardedDisplayContent } from '../forwardedEmailDisplay';

describe('cleanForwardedDisplayContent', () => {
  it('keeps the forwarded customer body while hiding Gmail forwarded header metadata', () => {
    const result = cleanForwardedDisplayContent(`---------- Forwarded message ---------
From: Jason Smith <jason@the-web-dev.com>
Date: Sun, May 31, 2026 at 2:38 PM
Subject: Data Export
To: Waqar from Usermaven <waqar@usermaven.com>

Hi Usermaven team,

I need summarised analytics data per domain.

--
Jason Smith`);

    expect(result).toContain('Hi Usermaven team,');
    expect(result).toContain('I need summarised analytics data per domain.');
    expect(result).not.toContain('Forwarded message');
    expect(result).not.toContain('From: Jason Smith');
    expect(result).not.toContain('Subject: Data Export');
  });

  it('keeps a teammate note above the forwarded header', () => {
    const result = cleanForwardedDisplayContent(`Please handle this.

---------- Forwarded message ---------
From: Jason Smith <jason@the-web-dev.com>
Date: Sun, May 31, 2026 at 2:38 PM
Subject: Data Export
To: Waqar from Usermaven <waqar@usermaven.com>

Can I export my data?`);

    expect(result).toBe('Please handle this.\n\nCan I export my data?');
  });
});
