(function () {
    const API_BASE = '/api/v1';
    const params = new URLSearchParams(window.location.search);
    const forced = params.get('backend');
    const stored = localStorage.getItem('periphera_use_backend');
    const USE_BACKEND = forced === 'false' ? false : forced === 'true' ? true : stored === 'false' ? false : true;

    function buildURL(path, query) {
        const url = new URL(API_BASE + path, window.location.origin);
        Object.entries(query || {}).forEach(([key, value]) => {
            if (value === undefined || value === null || value === '') return;
            if (Array.isArray(value)) value = value.join(',');
            url.searchParams.set(key, value);
        });
        return url.toString();
    }

    async function apiRequest(path, options) {
        if (!USE_BACKEND) throw new Error('backend disabled');
        const opts = Object.assign({ method: 'GET', params: null, body: undefined }, options || {});
        const fetchOptions = {
            method: opts.method,
            headers: { 'Accept': 'application/json' },
        };
        if (opts.body !== undefined) {
            fetchOptions.headers['Content-Type'] = 'application/json';
            fetchOptions.body = JSON.stringify(opts.body);
        }
        const res = await fetch(buildURL(path, opts.params), fetchOptions);
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const payload = await res.json();
        if (!payload || payload.code !== 0) {
            throw new Error((payload && payload.msg) || 'API failure');
        }
        return payload.data;
    }

    function apiGet(path, params) {
        return apiRequest(path, { method: 'GET', params: params || null });
    }

    function usePolling(fetcher, intervalMs, options) {
        const opt = Object.assign({ immediate: true, onError: null }, options || {});
        let timer = null;
        let stopped = false;
        let running = false;

        async function run() {
            if (stopped || document.hidden || running) return;
            running = true;
            try {
                await fetcher();
            } catch (err) {
                if (opt.onError) opt.onError(err);
                else console.warn('[PERIPHERA] polling failed:', err);
            } finally {
                running = false;
            }
        }

        function start() {
            if (timer || stopped) return;
            if (opt.immediate) run();
            timer = setInterval(run, intervalMs);
        }

        function stop() {
            if (timer) clearInterval(timer);
            timer = null;
        }

        function onVisibility() {
            if (document.hidden) stop();
            else {
                start();
                run();
            }
        }

        document.addEventListener('visibilitychange', onVisibility);
        start();

        return function dispose() {
            stopped = true;
            stop();
            document.removeEventListener('visibilitychange', onVisibility);
        };
    }

    function relativeTime(ts) {
        if (!ts) return '';
        const diff = Math.max(0, Date.now() - Number(ts));
        const minute = 60 * 1000;
        const hour = 60 * minute;
        const day = 24 * hour;
        if (diff < 45 * 1000) return '刚刚';
        if (diff < hour) return Math.floor(diff / minute) + ' 分钟前';
        if (diff < day) return Math.floor(diff / hour) + ' 小时前';
        return Math.floor(diff / day) + ' 天前';
    }

    function markFresh(item, ms) {
        item._new = true;
        setTimeout(() => { item._new = false; }, ms || 2600);
        return item;
    }

    function mergeById(current, incoming, options) {
        const opt = Object.assign({ prepend: true, limit: 60, fresh: false, idKey: 'uid' }, options || {});
        const seen = new Map(current.map((item, idx) => [item[opt.idKey], idx]));
        const added = [];
        incoming.forEach(item => {
            const id = item[opt.idKey];
            if (seen.has(id)) {
                current.splice(seen.get(id), 1);
            } else {
                added.push(item);
            }
            if (opt.fresh) markFresh(item);
            if (opt.prepend) current.unshift(item);
            else current.push(item);
        });
        if (current.length > opt.limit) current.length = opt.limit;
        return added.length;
    }

    function setBackend(enabled) {
        localStorage.setItem('periphera_use_backend', enabled ? 'true' : 'false');
        window.location.reload();
    }

    window.USE_BACKEND = USE_BACKEND;
    window.PeripheraAPI = {
        apiGet,
        apiRequest,
        usePolling,
        relativeTime,
        markFresh,
        mergeById,
        setBackend,
    };
})();
