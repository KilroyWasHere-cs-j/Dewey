import { describe, expect, it } from 'vitest';
import { buildLabelKey, parsePrometheusMetrics } from './metricsParser';

describe('buildLabelKey', () => {
	it('uses the bare value for a single label', () => {
		expect(buildLabelKey('op="read"')).toBe('read');
	});

	it('joins multiple labels as key=value pairs', () => {
		expect(buildLabelKey('quantile="0.5",code="200"')).toBe('quantile=0.5,code=200');
	});
});

describe('parsePrometheusMetrics', () => {
	it('parses a plain scalar metric', () => {
		const text = 'app_uptime_seconds 123.45\n';
		expect(parsePrometheusMetrics(text)).toEqual({ app_uptime_seconds: 123.45 });
	});

	it('strips the myapp_ namespace prefix from metric names', () => {
		const text = 'myapp_file_io_bytes_total{op="read"} 42\n';
		expect(parsePrometheusMetrics(text)).toEqual({
			file_io_bytes_total: { read: 42 }
		});
	});

	it('collapses a single-label metric in LABELED_METRICS to its bare value key', () => {
		const text = 'file_io_ops_total{op="write"} 7\n';
		expect(parsePrometheusMetrics(text)).toEqual({
			file_io_ops_total: { write: 7 }
		});
	});

	it('collapses a multi-label metric in LABELED_METRICS to a joined key', () => {
		const text = 'go_gc_duration_seconds{quantile="0.5",code="200"} 0.002\n';
		expect(parsePrometheusMetrics(text)).toEqual({
			go_gc_duration_seconds: { 'quantile=0.5,code=200': 0.002 }
		});
	});

	it('accumulates multiple label combinations for the same labeled metric', () => {
		const text = [
			'app_upload_rejections_total{reason="pe_blocked"} 3',
			'app_upload_rejections_total{reason="elf_blocked"} 1'
		].join('\n');
		expect(parsePrometheusMetrics(text)).toEqual({
			app_upload_rejections_total: { pe_blocked: 3, elf_blocked: 1 }
		});
	});

	it('silently drops a labeled metric that is not in LABELED_METRICS', () => {
		const text = 'some_unlisted_metric{op="read"} 5\n';
		expect(parsePrometheusMetrics(text)).toEqual({});
	});

	it('skips blank lines and comment lines', () => {
		const text = [
			'# HELP app_uptime_seconds Uptime of the application',
			'# TYPE app_uptime_seconds gauge',
			'',
			'app_uptime_seconds 10'
		].join('\n');
		expect(parsePrometheusMetrics(text)).toEqual({ app_uptime_seconds: 10 });
	});

	it('skips lines whose value is not a valid number', () => {
		const text = 'app_uptime_seconds not_a_number\n';
		expect(parsePrometheusMetrics(text)).toEqual({});
	});

	it('returns an empty object for empty input', () => {
		expect(parsePrometheusMetrics('')).toEqual({});
	});
});
