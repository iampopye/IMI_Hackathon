--
-- PostgreSQL database dump
--

\restrict cnBaNEV6cqt3AxGj65n5NhOhzDfWKaBDWhDC6LHtmmXFt79WVb1bN6FzQ1elvat

-- Dumped from database version 16.11
-- Dumped by pg_dump version 16.11

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: maintain_dataset_count(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.maintain_dataset_count() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.deleted_at IS NULL THEN
        INSERT INTO dataset_counts (dataset_id, record_count)
        VALUES (NEW.dataset_id, 1)
        ON CONFLICT (dataset_id)
        DO UPDATE SET
            record_count = dataset_counts.record_count + 1,
            updated_at   = NOW();

    ELSIF TG_OP = 'UPDATE' THEN
        -- Record soft-deleted
        IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
            UPDATE dataset_counts
            SET record_count = GREATEST(0, record_count - 1),
                updated_at   = NOW()
            WHERE dataset_id = NEW.dataset_id;
        END IF;
        -- Record restored from soft-delete
        IF OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL THEN
            UPDATE dataset_counts
            SET record_count = record_count + 1,
                updated_at   = NOW()
            WHERE dataset_id = NEW.dataset_id;
        END IF;
    END IF;
    RETURN NULL;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: dataset_access_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_access_log (
    id bigint NOT NULL,
    dataset_id uuid NOT NULL,
    accessed_at timestamp without time zone DEFAULT now()
);


--
-- Name: dataset_access_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.dataset_access_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: dataset_access_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.dataset_access_log_id_seq OWNED BY public.dataset_access_log.id;


--
-- Name: dataset_counts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_counts (
    dataset_id uuid NOT NULL,
    record_count bigint DEFAULT 0,
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: dataset_states; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_states (
    dataset_id uuid NOT NULL,
    is_sorted boolean DEFAULT false,
    last_modified timestamp without time zone DEFAULT now(),
    last_sorted timestamp without time zone,
    stability_score double precision DEFAULT 0.0,
    current_tier character varying(20) DEFAULT 'small'::character varying,
    is_sorting boolean DEFAULT false
);


--
-- Name: datasets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.datasets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    source character varying(255),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: outbox; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.outbox (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    event_type character varying(100) NOT NULL,
    dataset_id uuid NOT NULL,
    payload jsonb NOT NULL,
    status character varying(20) DEFAULT 'PENDING'::character varying,
    attempts integer DEFAULT 0,
    created_at timestamp without time zone DEFAULT now(),
    published_at timestamp without time zone
);


--
-- Name: records; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.records (
    id character varying(64) NOT NULL,
    dataset_id uuid NOT NULL,
    external_id character varying(255) NOT NULL,
    source character varying(255) NOT NULL,
    name text,
    value jsonb,
    checksum character varying(64) NOT NULL,
    version bigint DEFAULT 1,
    deleted_at timestamp without time zone,
    sync_token character varying(64),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying(255) NOT NULL,
    applied_at timestamp without time zone DEFAULT now()
);


--
-- Name: dataset_access_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_access_log ALTER COLUMN id SET DEFAULT nextval('public.dataset_access_log_id_seq'::regclass);


--
-- Name: dataset_access_log dataset_access_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_access_log
    ADD CONSTRAINT dataset_access_log_pkey PRIMARY KEY (id);


--
-- Name: dataset_counts dataset_counts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_counts
    ADD CONSTRAINT dataset_counts_pkey PRIMARY KEY (dataset_id);


--
-- Name: dataset_states dataset_states_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_states
    ADD CONSTRAINT dataset_states_pkey PRIMARY KEY (dataset_id);


--
-- Name: datasets datasets_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.datasets
    ADD CONSTRAINT datasets_name_key UNIQUE (name);


--
-- Name: datasets datasets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.datasets
    ADD CONSTRAINT datasets_pkey PRIMARY KEY (id);


--
-- Name: outbox outbox_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox
    ADD CONSTRAINT outbox_pkey PRIMARY KEY (id);


--
-- Name: records records_external_id_source_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.records
    ADD CONSTRAINT records_external_id_source_key UNIQUE (external_id, source);


--
-- Name: records records_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.records
    ADD CONSTRAINT records_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: idx_access_log_dataset; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_access_log_dataset ON public.dataset_access_log USING btree (dataset_id, accessed_at);


--
-- Name: idx_outbox_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_outbox_created_at ON public.outbox USING btree (created_at);


--
-- Name: idx_outbox_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_outbox_status ON public.outbox USING btree (status) WHERE ((status)::text = 'PENDING'::text);


--
-- Name: idx_records_active_by_dataset; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_records_active_by_dataset ON public.records USING btree (dataset_id, updated_at) WHERE (deleted_at IS NULL);


--
-- Name: idx_records_dataset_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_records_dataset_id ON public.records USING btree (dataset_id);


--
-- Name: idx_records_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_records_deleted_at ON public.records USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);


--
-- Name: idx_records_external_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_records_external_id ON public.records USING btree (external_id);


--
-- Name: idx_records_purge_lookup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_records_purge_lookup ON public.records USING btree (dataset_id, sync_token) WHERE (deleted_at IS NULL);


--
-- Name: idx_records_soft_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_records_soft_deleted ON public.records USING btree (dataset_id, deleted_at) WHERE (deleted_at IS NOT NULL);


--
-- Name: idx_records_sync_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_records_sync_token ON public.records USING btree (sync_token);


--
-- Name: idx_records_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_records_updated_at ON public.records USING btree (updated_at);


--
-- Name: records trg_dataset_count; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_dataset_count AFTER INSERT OR UPDATE ON public.records FOR EACH ROW EXECUTE FUNCTION public.maintain_dataset_count();


--
-- Name: dataset_counts dataset_counts_dataset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_counts
    ADD CONSTRAINT dataset_counts_dataset_id_fkey FOREIGN KEY (dataset_id) REFERENCES public.datasets(id);


--
-- Name: dataset_states dataset_states_dataset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_states
    ADD CONSTRAINT dataset_states_dataset_id_fkey FOREIGN KEY (dataset_id) REFERENCES public.datasets(id);


--
-- Name: records records_dataset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.records
    ADD CONSTRAINT records_dataset_id_fkey FOREIGN KEY (dataset_id) REFERENCES public.datasets(id);


--
-- PostgreSQL database dump complete
--

\unrestrict cnBaNEV6cqt3AxGj65n5NhOhzDfWKaBDWhDC6LHtmmXFt79WVb1bN6FzQ1elvat

