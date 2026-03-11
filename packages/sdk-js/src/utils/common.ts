import { getLogger } from './logger';

export type Breaker = {};
export type EventHandler = (event: Event) => boolean | void;
const ObjProto = Object.prototype;
const toString = ObjProto.toString;
const hasOwnProperty = ObjProto.hasOwnProperty;

const ArrayProto = Array.prototype;
const nativeForEach = ArrayProto.forEach,
  nativeIndexOf = ArrayProto.indexOf,
  nativeIsArray = Array.isArray,
  breaker: Breaker = {};

export const _isArray =
  nativeIsArray ||
  function (obj: any): obj is any[] {
    return toString.call(obj) === '[object Array]';
  };

export function _eachArray<E = any>(
  obj: E[] | null | undefined,
  iterator: (value: E, key: number) => void | Breaker,
  thisArg?: any,
): void {
  if (Array.isArray(obj)) {
    if (nativeForEach && obj.forEach === nativeForEach) {
      obj.forEach(iterator, thisArg);
    } else if ('length' in obj && obj.length === +obj.length) {
      for (let i = 0, l = obj.length; i < l; i++) {
        if (i in obj && iterator.call(thisArg, obj[i], i) === breaker) {
          return;
        }
      }
    }
  }
}
// Embed part of the Underscore Library
export const _trim = function (str: string): string {
  return str.replace(/^[\s\uFEFF\xA0]+|[\s\uFEFF\xA0]+$/g, '');
};

export const _bind_instance_methods = function (
  obj: Record<string, any>,
): void {
  for (const func in obj) {
    if (typeof obj[func] === 'function') {
      obj[func] = obj[func].bind(obj);
    }
  }
};

/**
 * @param {*=} obj
 * @param {function(...*)=} iterator
 * @param {Object=} thisArg
 */
export function _each(
  obj: any,
  iterator: (value: any, key: any) => void | Breaker,
  thisArg?: any,
): void {
  if (obj === null || obj === undefined) {
    return;
  }
  if (nativeForEach && Array.isArray(obj) && obj.forEach === nativeForEach) {
    obj.forEach(iterator, thisArg);
  } else if ('length' in obj && obj.length === +obj.length) {
    for (let i = 0, l = obj.length; i < l; i++) {
      if (i in obj && iterator.call(thisArg, obj[i], i) === breaker) {
        return;
      }
    }
  } else {
    for (const key in obj) {
      if (hasOwnProperty.call(obj, key)) {
        if (iterator.call(thisArg, obj[key], key) === breaker) {
          return;
        }
      }
    }
  }
}
export const _extend = function (
  obj: Record<string, any>,
  ...args: Record<string, any>[]
): Record<string, any> {
  _eachArray(args, function (source) {
    for (const prop in source) {
      if (source[prop] !== void 0) {
        obj[prop] = source[prop];
      }
    }
  });
  return obj;
};
export function _includes<T = any>(str: T[] | string, needle: T): boolean {
  return (str as any).indexOf(needle) !== -1;
}

// from a comment on http://dbj.org/dbj/?p=286
// fails on only one very rare and deliberate custom object:
// let bomb = { toString : undefined, valueOf: function(o) { return "function BOMBA!"; }};
export const _isFunction = function (f: any): f is (...args: any[]) => any {
  try {
    return /^\s*\bfunction\b/.test(f);
  } catch (x) {
    return false;
  }
};
export const _isUndefined = function (obj: any): obj is undefined {
  return obj === void 0;
};

export const _register_event = (function () {
  // written by Dean Edwards, 2005
  // with input from Tino Zijdel - crisp@xs4all.nl
  // with input from Carl Sverre - mail@carlsverre.com
  // with input from PostHog
  // http://dean.edwards.name/weblog/2005/10/add-event/
  // https://gist.github.com/1930440

  /**
   * @param {Object} element
   * @param {string} type
   * @param {function(...*)} handler
   * @param {boolean=} oldSchool
   * @param {boolean=} useCapture
   */
  const register_event = function (
    element: Element | Window | Document | Node,
    type: string,
    handler: EventHandler,
    oldSchool?: boolean,
    useCapture?: boolean,
  ) {
    if (!element) {
      getLogger().error('No valid element provided to register_event');
      return;
    }

    if (element.addEventListener && !oldSchool) {
      // @ts-ignore
      element.addEventListener(type, handler, !!useCapture);
    } else {
      const ontype = 'on' + type;
      const old_handler = (element as any)[ontype]; // can be undefined
      (element as any)[ontype] = makeHandler(element, handler, old_handler);
    }
  };

  function makeHandler(
    element: Element | Window | Document | Node,
    new_handler: EventHandler,
    old_handlers: EventHandler,
  ) {
    return function (event: Event): boolean | void {
      event = event || fixEvent(window.event);

      // this basically happens in firefox whenever another scripts
      // overwrites the onload callback and doesn't pass the event
      // object to previously defined callbacks.  All the browsers
      // that don't define window.event implement addEventListener
      // so the dom_loaded handler will still be fired as usual.
      if (!event) {
        return undefined;
      }

      let ret = true;
      let old_result: any;

      if (_isFunction(old_handlers)) {
        old_result = old_handlers(event);
      }
      const new_result = new_handler.call(element, event);

      if (false === old_result || false === new_result) {
        ret = false;
      }

      return ret;
    };
  }

  function fixEvent(event: Event | undefined): Event | undefined {
    if (event) {
      event.preventDefault = fixEvent.preventDefault;
      event.stopPropagation = fixEvent.stopPropagation;
    }
    return event;
  }
  fixEvent.preventDefault = function () {
    (this as any as Event).returnValue = false;
  };
  fixEvent.stopPropagation = function () {
    (this as any as Event).cancelBubble = true;
  };

  return register_event;
})();

export const _safewrap = function <
  F extends (...args: any[]) => any = (...args: any[]) => any,
>(f: F): F {
  return function (...args) {
    try {
      // eslint-disable-next-line @typescript-eslint/ban-ts-comment
      // @ts-ignore
      return f.apply(this, args);
    } catch (e) {
      getLogger().error(
        'Implementation error. Please turn on debug and contact support@helpin.ai.',
        e,
      );
      // if (Config.DEBUG) {
      //     getLogger.critical(e)
      // }
    }
  } as F;
};

export const _safewrap_instance_methods = function (
  obj: Record<string, any>,
): void {
  for (const func in obj) {
    if (typeof obj[func] === 'function') {
      obj[func] = _safewrap(obj[func]);
    }
  }
};

const COPY_IN_PROGRESS_ATTRIBUTE =
  typeof Symbol !== 'undefined'
    ? Symbol('__deepCircularCopyInProgress__')
    : '__deepCircularCopyInProgress__';

/**
 * Deep copies an object.
 * It handles cycles by replacing all references to them with `undefined`
 * Also supports customizing native values
 *
 * @param value
 * @param customizer
 * @param [key] if provided this is the object key associated with the value to be copied. It allows the customizer function to have context when it runs
 * @returns {{}|undefined|*}
 */
function deepCircularCopy<T extends Record<string, any> = Record<string, any>>(
  value: T,
  customizer?: <K extends keyof T = keyof T>(value: T[K], key?: K) => T[K],
  key?: string,
): T | undefined {
  if (value !== Object(value))
    return customizer ? customizer(value as any, key) : value; // primitive value

  if (value[COPY_IN_PROGRESS_ATTRIBUTE as any]) return undefined;
  (value as any)[COPY_IN_PROGRESS_ATTRIBUTE] = true;
  let result: T;

  if (_isArray(value)) {
    result = [] as any as T;
    _eachArray(value, (it) => {
      result.push(deepCircularCopy(it, customizer));
    });
  } else {
    result = {} as T;
    _each(value, (val, key) => {
      if (key !== COPY_IN_PROGRESS_ATTRIBUTE) {
        (result as any)[key] = deepCircularCopy(val, customizer, key);
      }
    });
  }
  delete value[COPY_IN_PROGRESS_ATTRIBUTE as any];
  return result;
}

const LONG_STRINGS_ALLOW_LIST = ['$performance_raw'];

export function _copyAndTruncateStrings<
  T extends Record<string, any> = Record<string, any>,
>(object: T, maxStringLength: number | null): T {
  return deepCircularCopy(object, (value: any, key) => {
    if (key && LONG_STRINGS_ALLOW_LIST.indexOf(key as string) > -1) {
      return value;
    }
    if (typeof value === 'string' && maxStringLength !== null) {
      return (value as string).slice(0, maxStringLength);
    }
    return value;
  }) as T;
}

// This is to block various web spiders from executing our JS and
// sending false capturing data
export const _isBlockedUA = function (ua: string): boolean {
  if (
    /(google web preview|baiduspider|yandexbot|bingbot|googlebot|yahoo! slurp|ahrefsbot|facebookexternalhit|facebookcatalog)/i.test(
      ua,
    )
  ) {
    return true;
  }
  return false;
};

// Function to find the closest link element
export function _findClosestLink(
  element: HTMLElement | null,
): HTMLElement | null {
  while (element && element.tagName) {
    if (element.tagName.toLowerCase() == 'a') {
      return element;
    }
    element = element.parentNode as HTMLElement | null;
  }
  return null;
}

export function _cleanObject(obj: Record<string, any>) {
  for (let propName in obj) {
    if (
      obj[propName] === '' ||
      obj[propName] === null ||
      obj[propName] === undefined ||
      (typeof obj[propName] === 'object' &&
        Object.keys(obj[propName]).length === 0)
    ) {
      delete obj[propName];
    }
  }
  return obj;
}

export function isWindowAvailable(): boolean {
  try {
    return (
      typeof window !== 'undefined' &&
      window.document !== undefined &&
      window.document.createElement !== undefined
    );
  } catch (e) {
    getLogger().warn('window is not available');
    return false;
  }
}

export function generateRandom(length: number = 5): string {
  const array = new Uint8Array(length);
  crypto.getRandomValues(array);
  return Array.from(array, (byte) => byte.toString(36).padStart(2, '0'))
    .join('')
    .slice(0, length);
}

export function toCamelCase(str: string): string {
  return str.replace(/([-_][a-z])/g, (group) =>
    group.toUpperCase().replace('-', '').replace('_', ''),
  );
}

export function convertKeysToCamelCase(
  obj: Record<string, any>,
): Record<string, any> {
  if (typeof obj !== 'object' || obj === null) {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(convertKeysToCamelCase);
  }

  return Object.keys(obj).reduce(
    (result, key) => {
      const camelKey = toCamelCase(key);
      result[camelKey] = convertKeysToCamelCase(obj[key]);
      return result;
    },
    {} as Record<string, any>,
  );
}
