import { notFound } from 'next/navigation';
import { createPageMetadata } from '@/lib/metadata';
import { AlternativesPage } from '../AlternativesPage';
import { ALTERNATIVES, alternativesSeo } from '../alternatives-data';
import { ComparePage } from '../ComparePage';
import { COMPETITORS, competitorSeo } from '../compare-data';
import '../../_components/platform/platform.css';
import '../../_components/platform/platform-polish.css';
import '../compare.css';

// One route serves both "Helpin vs X" comparisons and "X alternatives" lists.
export const dynamicParams = false;

export function generateStaticParams() {
  return [...COMPETITORS, ...ALTERNATIVES].map(page => ({ slug: page.slug }));
}

type Params = Promise<{ slug: string }>;

export async function generateMetadata({ params }: { params: Params }) {
  const { slug } = await params;
  const competitor = COMPETITORS.find(item => item.slug === slug);
  if (competitor) return createPageMetadata(competitorSeo(competitor));
  const list = ALTERNATIVES.find(item => item.slug === slug);
  return list ? createPageMetadata(alternativesSeo(list)) : {};
}

export default async function ComparisonRoute({ params }: { params: Params }) {
  const { slug } = await params;
  const competitor = COMPETITORS.find(item => item.slug === slug);
  if (competitor) return <ComparePage competitor={competitor} />;
  const list = ALTERNATIVES.find(item => item.slug === slug);
  if (list) return <AlternativesPage page={list} />;
  notFound();
}
