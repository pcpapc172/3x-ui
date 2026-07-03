import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, InputNumber, Layout, Popconfirm, Select, Space, Table, message } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import type { Msg } from '@/utils';

interface MultiplyEntry {
  id: number;
  inboundId: number;
  inboundTag: string;
  rate: number;
}

interface InboundOption {
  id: number;
  remark?: string;
  tag?: string;
  protocol?: string;
  port?: number;
}

export default function MultiplyPage() {
  const { t } = useTranslation();
  const [entries, setEntries] = useState<MultiplyEntry[]>([]);
  const [inbounds, setInbounds] = useState<InboundOption[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedInbound, setSelectedInbound] = useState<number | null>(null);
  const [rate, setRate] = useState<number>(1);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [multResp, inbResp] = await Promise.all([
        HttpUtil.get<Msg<MultiplyEntry[]>>('/panel/api/multiply/list'),
        HttpUtil.get<Msg<InboundOption[]>>('/panel/api/inbounds/options'),
      ]);
      if (multResp.success && multResp.obj) setEntries(multResp.obj);
      if (inbResp.success && inbResp.obj) setInbounds(inbResp.obj);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleAdd = async () => {
    if (!selectedInbound) return;
    const resp = await HttpUtil.post('/panel/api/multiply/set', {
      inboundId: selectedInbound,
      rate,
    });
    if (resp.success) {
      setSelectedInbound(null);
      setRate(1);
      fetchData();
    }
  };

  const handleDelete = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/multiply/del/${id}`);
    if (resp.success) fetchData();
  };

  const columns = [
    {
      title: 'Inbound',
      dataIndex: 'inboundTag',
      key: 'inboundTag',
      render: (tag: string) => tag || 'Unknown',
    },
    {
      title: 'Rate',
      dataIndex: 'rate',
      key: 'rate',
      render: (rate: number) => <strong>{rate}x</strong>,
    },
    {
      title: '',
      key: 'actions',
      width: 60,
      render: (_: unknown, record: MultiplyEntry) => (
        <Popconfirm title="Remove this multiplier?" onConfirm={() => handleDelete(record.id)}>
          <Button size="small" danger icon={<DeleteOutlined />} />
        </Popconfirm>
      ),
    },
  ];

  const usedInboundIds = new Set(entries.map((e) => e.inboundId));
  const availableInbounds = inbounds.filter((ib) => !usedInboundIds.has(ib.id));

  return (
    <Layout.Content style={{ padding: 24 }}>
      <Card title="Traffic Multiplier" extra={
        <Space>
          <Select
            style={{ width: 200 }}
            placeholder="Select inbound"
            value={selectedInbound}
            onChange={setSelectedInbound}
            options={availableInbounds.map((ib) => ({
              value: ib.id,
              label: `${ib.remark || ib.tag || ib.id} (${ib.protocol}:${ib.port})`,
            }))}
          />
          <InputNumber
            min={0.1}
            step={0.1}
            value={rate}
            onChange={(v) => setRate(v ?? 1)}
            addonAfter="x"
            style={{ width: 100 }}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd} disabled={!selectedInbound}>
            Add
          </Button>
        </Space>
      }>
        <Table
          dataSource={entries}
          columns={columns}
          rowKey="id"
          loading={loading}
          pagination={false}
        />
      </Card>
    </Layout.Content>
  );
}
