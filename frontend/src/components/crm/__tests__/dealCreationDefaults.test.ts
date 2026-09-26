import { describe, expect, it } from 'vitest';
import { recurringRevenue, generatedDealName, resolveDealStage, comparableDealTotal } from '../dealCreationDefaults';

describe('deal defaults', () => {
 it('does not combine different currencies or periods into misleading totals', () => {
 expect(comparableDealTotal([{amount:100,currency:'USD',revenue_type:'monthly'},{amount:200,currency:'USD',revenue_type:'annual'}])).toBeNull();
 expect(comparableDealTotal([{amount:100,currency:'USD'},{amount:200,currency:'EUR'}])).toBeNull();
 expect(comparableDealTotal([{amount:100,currency:'USD',revenue_type:'monthly'},{amount:200,currency:'USD',revenue_type:'monthly'}])).toBe('USD 300 / mo');
 });
 it('uses company then contact without inventing a customer', () => {
 expect(generatedDealName('Acme', 'Alex')).toBe('Acme — New deal');
 expect(generatedDealName(undefined, 'Alex')).toBe('Alex — New deal');
 expect(generatedDealName()).toBe('');
 });
 it('inherits the default pipeline and checks stage membership', () => {
 const pipelines = [{id:'other',is_default:false,stages:[]}, {id:'default',is_default:true,stages:[{id:'b',position:1,probability:50},{id:'a',position:0,probability:20}]}];
 expect(resolveDealStage(pipelines)).toMatchObject({pipelineId:'default',stageId:'a',probability:20});
 expect(resolveDealStage(pipelines,'default','b').stageId).toBe('b');
 expect(resolveDealStage(pipelines,'default','foreign').stageId).toBe('a');
 });
 it('normalizes recurring amounts and excludes one-time amounts', () => {
 expect(recurringRevenue(1200,'annual')).toEqual({mrr:100,arr:1200});
 expect(recurringRevenue(100,'monthly')).toEqual({mrr:100,arr:1200});
 expect(recurringRevenue(100,'one_time')).toBeNull();
 expect(recurringRevenue(undefined,'monthly')).toBeNull();
 expect(recurringRevenue(0,'monthly')).toEqual({mrr:0,arr:0});
 });
});
