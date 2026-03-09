import { useState, useCallback } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Upload, FileText, ArrowRight, Check, AlertCircle } from 'lucide-react';
import { toast } from 'sonner';
import { useCreateCRMImport, useProcessCRMImport } from '@/hooks/queries/useCRM';
import type { CRMObjectType, ImportColumnMapping, CRMImportJob } from '@/lib/crmTypes';

type WizardStep = 'upload' | 'map' | 'preview' | 'importing' | 'results';

const CONTACT_FIELDS = ['first_name', 'last_name', 'email', 'phone', 'job_title', 'lifecycle_stage', 'lead_status', 'source'];
const COMPANY_FIELDS = ['name', 'domain', 'industry', 'description'];
const DEAL_FIELDS = ['name', 'pipeline_id', 'stage_id', 'amount', 'currency'];

interface CRMImportWizardProps {
  workspaceId: string;
  onComplete?: () => void;
}

export function CRMImportWizard({ workspaceId, onComplete }: CRMImportWizardProps) {
  const [step, setStep] = useState<WizardStep>('upload');
  const [objectType, setObjectType] = useState<CRMObjectType>('contact');
  const [csvHeaders, setCsvHeaders] = useState<string[]>([]);
  const [csvRows, setCsvRows] = useState<string[][]>([]);
  const [columnMapping, setColumnMapping] = useState<ImportColumnMapping[]>([]);
  const [result, setResult] = useState<CRMImportJob | null>(null);

  const createImport = useCreateCRMImport(workspaceId);
  const processImport = useProcessCRMImport(workspaceId);

  const availableFields = objectType === 'contact' ? CONTACT_FIELDS :
    objectType === 'company' ? COMPANY_FIELDS : DEAL_FIELDS;

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (ev) => {
      const text = ev.target?.result as string;
      const lines = text.split('\n').map((line) => line.trim()).filter(Boolean);
      if (lines.length < 2) {
        toast.error('CSV must have at least a header row and one data row');
        return;
      }

      const headers = parseCSVLine(lines[0]);
      const rows = lines.slice(1).map(parseCSVLine);

      setCsvHeaders(headers);
      setCsvRows(rows);

      // Auto-map columns by name matching
      const mapping: ImportColumnMapping[] = headers.map((header) => {
        const normalized = header.toLowerCase().replace(/\s+/g, '_');
        const matched = availableFields.find((f) => f === normalized);
        return {
          csv_column: header,
          crm_field: matched ?? '',
          is_custom: false,
        };
      });
      setColumnMapping(mapping);
      setStep('map');
    };
    reader.readAsText(file);
  }, [availableFields]);

  const handleMapChange = (index: number, field: string) => {
    const updated = [...columnMapping];
    updated[index] = { ...updated[index], crm_field: field, is_custom: !availableFields.includes(field) && field !== '' };
    setColumnMapping(updated);
  };

  const handleImport = async () => {
    setStep('importing');
    try {
      // Create import job
      const job = await createImport.mutateAsync({
        workspace_id: workspaceId,
        source: 'csv',
        object_type: objectType,
        total_rows: csvRows.length,
      });

      // Process import with data
      const processedJob = await processImport.mutateAsync({
        id: job.id,
        column_mapping: columnMapping,
        csv_data: csvRows,
      });

      setResult(processedJob);
      setStep('results');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Import failed');
      setStep('preview');
    }
  };

  return (
    <div className="space-y-6">
      {/* Step indicator */}
      <div className="flex items-center gap-2">
        <StepIndicator step="upload" current={step} label="Upload" />
        <ArrowRight className="h-4 w-4 text-muted-foreground" />
        <StepIndicator step="map" current={step} label="Map Columns" />
        <ArrowRight className="h-4 w-4 text-muted-foreground" />
        <StepIndicator step="preview" current={step} label="Preview" />
        <ArrowRight className="h-4 w-4 text-muted-foreground" />
        <StepIndicator step="results" current={step} label="Results" />
      </div>

      {step === 'upload' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Upload CSV File</CardTitle>
            <CardDescription>Select the type of records and upload your CSV file</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label>Record Type</Label>
              <Select value={objectType} onValueChange={(v) => setObjectType(v as CRMObjectType)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="contact">Contacts</SelectItem>
                  <SelectItem value="company">Companies</SelectItem>
                  <SelectItem value="deal">Deals</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>CSV File</Label>
              <div className="border-2 border-dashed rounded-lg p-8 text-center">
                <Upload className="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
                <p className="text-sm text-muted-foreground mb-3">Drag and drop or click to upload</p>
                <Input
                  type="file"
                  accept=".csv"
                  onChange={handleFileUpload}
                  className="max-w-xs mx-auto"
                />
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 'map' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Map Columns</CardTitle>
            <CardDescription>
              Map your CSV columns to CRM fields. {csvRows.length} row(s) detected.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>CSV Column</TableHead>
                  <TableHead>Sample Value</TableHead>
                  <TableHead>CRM Field</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {csvHeaders.map((header, idx) => (
                  <TableRow key={idx}>
                    <TableCell className="font-medium">
                      <FileText className="h-3.5 w-3.5 inline mr-1 text-muted-foreground" />
                      {header}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {csvRows[0]?.[idx] ?? '-'}
                    </TableCell>
                    <TableCell>
                      <Select
                        value={columnMapping[idx]?.crm_field ?? ''}
                        onValueChange={(v) => handleMapChange(idx, v)}
                      >
                        <SelectTrigger className="w-[200px]">
                          <SelectValue placeholder="Skip this column" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="">Skip</SelectItem>
                          {availableFields.map((field) => (
                            <SelectItem key={field} value={field}>{field}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="flex justify-between mt-4">
              <Button variant="outline" onClick={() => setStep('upload')}>Back</Button>
              <Button onClick={() => setStep('preview')}>Preview</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 'preview' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Preview Import</CardTitle>
            <CardDescription>Review the first few rows before importing</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="mb-4 space-y-1">
              <p className="text-sm"><strong>Type:</strong> {objectType}</p>
              <p className="text-sm"><strong>Total rows:</strong> {csvRows.length}</p>
              <p className="text-sm"><strong>Mapped fields:</strong> {columnMapping.filter((m) => m.crm_field).length} of {csvHeaders.length}</p>
            </div>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    {columnMapping.filter((m) => m.crm_field).map((m, idx) => (
                      <TableHead key={idx}>{m.crm_field}</TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {csvRows.slice(0, 5).map((row, rowIdx) => (
                    <TableRow key={rowIdx}>
                      {columnMapping.map((m, colIdx) => {
                        if (!m.crm_field) return null;
                        return <TableCell key={colIdx} className="text-sm">{row[colIdx] ?? ''}</TableCell>;
                      })}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            {csvRows.length > 5 && (
              <p className="text-xs text-muted-foreground mt-2">Showing 5 of {csvRows.length} rows</p>
            )}
            <div className="flex justify-between mt-4">
              <Button variant="outline" onClick={() => setStep('map')}>Back</Button>
              <Button onClick={handleImport}>Import {csvRows.length} Records</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {step === 'importing' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Importing...</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <Progress value={50} />
            <p className="text-sm text-muted-foreground text-center">Processing your records...</p>
          </CardContent>
        </Card>
      )}

      {step === 'results' && result && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              {result.status === 'completed' ? (
                <><Check className="h-5 w-5 text-green-500" /> Import Complete</>
              ) : (
                <><AlertCircle className="h-5 w-5 text-destructive" /> Import Failed</>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">Total Rows</p>
                <p className="text-lg font-semibold">{result.total_rows}</p>
              </div>
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">Processed</p>
                <p className="text-lg font-semibold">{result.processed_rows}</p>
              </div>
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">Created</p>
                <p className="text-lg font-semibold text-green-600">{result.created_rows}</p>
              </div>
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">Errors</p>
                <p className="text-lg font-semibold text-destructive">{result.error_count}</p>
              </div>
            </div>
            {result.error_count > 0 && (
              <div className="bg-destructive/10 rounded-md p-3">
                <p className="text-sm font-medium text-destructive mb-1">Errors:</p>
                {Object.values(result.error_log).slice(0, 10).map((entry, idx) => {
                  const e = entry as Record<string, unknown>;
                  return (
                    <p key={idx} className="text-xs text-destructive">
                      Row {String(e.row)}: {String(e.error)}
                    </p>
                  );
                })}
              </div>
            )}
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="outline" onClick={() => { setStep('upload'); setCsvHeaders([]); setCsvRows([]); setResult(null); }}>
                Import More
              </Button>
              {onComplete && <Button onClick={onComplete}>Done</Button>}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function StepIndicator({ step, current, label }: { step: WizardStep; current: WizardStep; label: string }) {
  const steps: WizardStep[] = ['upload', 'map', 'preview', 'importing', 'results'];
  const stepIdx = steps.indexOf(step);
  const currentIdx = steps.indexOf(current);
  const isActive = step === current;
  const isDone = stepIdx < currentIdx;

  return (
    <Badge variant={isActive ? 'default' : isDone ? 'secondary' : 'outline'} className="text-xs">
      {isDone && <Check className="h-3 w-3 mr-1" />}
      {label}
    </Badge>
  );
}

function parseCSVLine(line: string): string[] {
  const result: string[] = [];
  let current = '';
  let inQuotes = false;

  for (let i = 0; i < line.length; i++) {
    const char = line[i];
    if (char === '"') {
      if (inQuotes && line[i + 1] === '"') {
        current += '"';
        i++;
      } else {
        inQuotes = !inQuotes;
      }
    } else if (char === ',' && !inQuotes) {
      result.push(current.trim());
      current = '';
    } else {
      current += char;
    }
  }
  result.push(current.trim());
  return result;
}
